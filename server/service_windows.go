// Copyright (c) 2013-2016 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package server

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/btcsuite/winsvc/eventlog"
	"github.com/btcsuite/winsvc/mgr"
	"github.com/btcsuite/winsvc/svc"
)

const (
	// SvcName is the name of btcd service.
	SvcName = "podsvc"

	// SvcDisplayName is the service name that will be shown in the windows
	// services list.  Not the SvcName is the "real" name which is used
	// to control the service.  This is only for display purposes.
	SvcDisplayName = "Pod Service"

	// SvcDesc is the description of the service.
	SvcDesc = "Downloads and stays synchronized with the parallelcoin block " +
		"chain and provides chain services to applications."
)

// Elog is used to send messages to the Windows event log.
var Elog *eventlog.Log

// LogServiceStartOfDay logs information about btcd when the main server has
// been started to the Windows event log.
func LogServiceStartOfDay(srvr *server) {
	var message string
	message += fmt.Sprintf("Version %s\n", version())
	message += fmt.Sprintf("Configuration directory: %s\n", defaultHomeDir)
	message += fmt.Sprintf("Configuration file: %s\n", cfg.ConfigFile)
	message += fmt.Sprintf("Data directory: %s\n", cfg.DataDir)

	Elog.Info(1, message)
}

// podService houses the main service handler which handles all service
// updates and launching btcdMain.
type podService struct{}

// Execute is the main entry point the winsvc package calls when receiving
// information from the Windows service control manager.  It launches the
// long-running btcdMain (which is the real meat of btcd), handles service
// change requests, and notifies the service control manager of changes.
func (s *podService) Execute(args []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (bool, uint32) {
	// Service start is pending.
	const cmdsAccepted = svc.AcceptStop | svc.AcceptShutdown
	changes <- svc.Status{State: svc.StartPending}

	// Start btcdMain in a separate goroutine so the service can start
	// quickly.  Shutdown (along with a potential error) is reported via
	// doneChan.  serverChan is notified with the main server instance once
	// it is started so it can be gracefully stopped.
	doneChan := make(chan error)
	serverChan := make(chan *server)
	go func() {
		err := btcdMain(serverChan)
		doneChan <- err
	}()

	// Service is now started.
	changes <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}

	var mainServer *server
loop:
	for {
		select {
		case c := <-r:
			switch c.Cmd {
			case svc.Interrogate:
				changes <- c.CurrentStatus

			case svc.Stop, svc.Shutdown:
				// Service stop is pending.  Don't accept any
				// more commands while pending.
				changes <- svc.Status{State: svc.StopPending}

				// Signal the main function to exit.
				shutdownRequestChannel <- struct{}{}

			default:
				Elog.Error(1, fmt.Sprintf("Unexpected control "+
					"request #%d.", c))
			}

		case srvr := <-serverChan:
			mainServer = srvr
			LogServiceStartOfDay(mainServer)

		case err := <-doneChan:
			if err != nil {
				Elog.Error(1, err.Error())
			}
			break loop
		}
	}

	// Service is now stopped.
	changes <- svc.Status{State: svc.Stopped}
	return false, 0
}

// InstallService attempts to install the btcd service.  Typically this should
// be done by the msi installer, but it is provided here since it can be useful
// for development.
func InstallService() error {
	// Get the path of the current executable.  This is needed because
	// os.Args[0] can vary depending on how the application was launched.
	// For example, under cmd.exe it will only be the name of the app
	// without the path or extension, but under mingw it will be the full
	// path including the extension.
	exePath, err := filepath.Abs(os.Args[0])
	if err != nil {
		return err
	}
	if filepath.Ext(exePath) == "" {
		exePath += ".exe"
	}

	// Connect to the windows service manager.
	serviceManager, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer serviceManager.Disconnect()

	// Ensure the service doesn't already exist.
	service, err := serviceManager.OpenService(SvcName)
	if err == nil {
		service.Close()
		return fmt.Errorf("service %s already exists", SvcName)
	}

	// Install the service.
	service, err = serviceManager.CreateService(SvcName, exePath, mgr.Config{
		DisplayName: SvcDisplayName,
		Description: SvcDesc,
	})
	if err != nil {
		return err
	}
	defer service.Close()

	// Support events to the event log using the standard "standard" Windows
	// EventCreate.exe message file.  This allows easy logging of custom
	// messges instead of needing to create our own message catalog.
	eventlog.Remove(SvcName)
	eventsSupported := uint32(eventlog.Error | eventlog.Warning | eventlog.Info)
	return eventlog.InstallAsEventCreate(SvcName, eventsSupported)
}

// RemoveService attempts to uninstall the btcd service.  Typically this should
// be done by the msi uninstaller, but it is provided here since it can be
// useful for development.  Not the eventlog entry is intentionally not removed
// since it would invalidate any existing event log messages.
func RemoveService() error {
	// Connect to the windows service manager.
	serviceManager, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer serviceManager.Disconnect()

	// Ensure the service exists.
	service, err := serviceManager.OpenService(SvcName)
	if err != nil {
		return fmt.Errorf("service %s is not installed", SvcName)
	}
	defer service.Close()

	// Remove the service.
	return service.Delete()
}

// StartService attempts to start the btcd service.
func StartService() error {
	// Connect to the windows service manager.
	serviceManager, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer serviceManager.Disconnect()

	service, err := serviceManager.OpenService(SvcName)
	if err != nil {
		return fmt.Errorf("could not access service: %v", err)
	}
	defer service.Close()

	err = service.Start(os.Args)
	if err != nil {
		return fmt.Errorf("could not start service: %v", err)
	}

	return nil
}

// ControlService allows commands which change the status of the service.  It
// also waits for up to 10 seconds for the service to change to the passed
// state.
func ControlService(c svc.Cmd, to svc.State) error {
	// Connect to the windows service manager.
	serviceManager, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer serviceManager.Disconnect()

	service, err := serviceManager.OpenService(SvcName)
	if err != nil {
		return fmt.Errorf("could not access service: %v", err)
	}
	defer service.Close()

	status, err := service.Control(c)
	if err != nil {
		return fmt.Errorf("could not send control=%d: %v", c, err)
	}

	// Send the control message.
	timeout := time.Now().Add(10 * time.Second)
	for status.State != to {
		if timeout.Before(time.Now()) {
			return fmt.Errorf("timeout waiting for service to go "+
				"to state=%d", to)
		}
		time.Sleep(300 * time.Millisecond)
		status, err = service.Query()
		if err != nil {
			return fmt.Errorf("could not retrieve service "+
				"status: %v", err)
		}
	}

	return nil
}

// PerformServiceCommand attempts to run one of the supported service commands
// provided on the command line via the service command flag.  An appropriate
// error is returned if an invalid command is specified.
func PerformServiceCommand(command string) error {
	var err error
	switch command {
	case "install":
		err = InstallService()

	case "remove":
		err = RemoveService()

	case "start":
		err = StartService()

	case "stop":
		err = ControlService(svc.Stop, svc.Stopped)

	default:
		err = fmt.Errorf("invalid service command [%s]", command)
	}

	return err
}

// ServiceMain checks whether we're being invoked as a service, and if so uses
// the service control manager to start the long-running server.  A flag is
// returned to the caller so the application can determine whether to exit (when
// running as a service) or launch in normal interactive mode.
func ServiceMain() (bool, error) {
	// Don't run as a service if we're running interactively (or that can't
	// be determined due to an error).
	isInteractive, err := svc.IsAnInteractiveSession()
	if err != nil {
		return false, err
	}
	if isInteractive {
		return false, nil
	}

	Elog, err = eventlog.Open(SvcName)
	if err != nil {
		return false, err
	}
	defer Elog.Close()

	err = svc.Run(SvcName, &podService{})
	if err != nil {
		Elog.Error(1, fmt.Sprintf("Service start failed: %v", err))
		return true, err
	}

	return true, nil
}

// Set windows specific functions to real functions.
func init() {
	runServiceCommand = PerformServiceCommand
	winServiceMain = ServiceMain
}
