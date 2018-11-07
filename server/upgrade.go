// Copyright (c) 2013-2014 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package server

import (
	"io"
	"os"
	"path/filepath"
)

// DirEmpty returns whether or not the specified directory path is empty.
func DirEmpty(dirPath string) (bool, error) {
	f, err := os.Open(dirPath)
	if err != nil {
		return false, err
	}
	defer f.Close()

	// Read the names of a max of one entry from the directory.  When the
	// directory is empty, an io.EOF error will be returned, so allow it.
	names, err := f.Readdirnames(1)
	if err != nil && err != io.EOF {
		return false, err
	}

	return len(names) == 0, nil
}

// OldBtcdHomeDir returns the OS specific home directory btcd used prior to
// version 0.3.3.  This has since been replaced with utils.AppDataDir, but
// this function is still provided for the automatic upgrade path.
func OldBtcdHomeDir() string {
	// Search for Windows APPDATA first.  This won't exist on POSIX OSes.
	appData := os.Getenv("APPDATA")
	if appData != "" {
		return filepath.Join(appData, "btcd")
	}

	// Fall back to standard HOME directory that works for most POSIX OSes.
	home := os.Getenv("HOME")
	if home != "" {
		return filepath.Join(home, ".btcd")
	}

	// In the worst case, use the current directory.
	return "."
}

// UpgradeDbPathNet moves the database for a specific network from its
// location prior to btcd version 0.2.0 and uses heuristics to ascertain the old
// database type to rename to the new format.
func UpgradeDbPathNet(oldDbPath, netName string) error {
	// Prior to version 0.2.0, the database was named the same thing for
	// both sqlite and leveldb.  Use heuristics to figure out the type
	// of the database and move it to the new path and name introduced with
	// version 0.2.0 accordingly.
	fi, err := os.Stat(oldDbPath)
	if err == nil {
		oldDbType := "sqlite"
		if fi.IsDir() {
			oldDbType = "leveldb"
		}

		// The new database name is based on the database type and
		// resides in a directory named after the network type.
		newDbRoot := filepath.Join(filepath.Dir(Cfg.DataDir), netName)
		newDbName := blockDbNamePrefix + "_" + oldDbType
		if oldDbType == "sqlite" {
			newDbName = newDbName + ".db"
		}
		newDbPath := filepath.Join(newDbRoot, newDbName)

		// Create the new path if needed.
		err = os.MkdirAll(newDbRoot, 0700)
		if err != nil {
			return err
		}

		// Move and rename the old database.
		err := os.Rename(oldDbPath, newDbPath)
		if err != nil {
			return err
		}
	}

	return nil
}

// UpgradeDbPaths moves the databases from their locations prior to btcd
// version 0.2.0 to their new locations.
func UpgradeDbPaths() error {
	// Prior to version 0.2.0, the databases were in the "db" directory and
	// their names were suffixed by "testnet" and "regtest" for their
	// respective networks.  Check for the old database and update it to the
	// new path introduced with version 0.2.0 accordingly.
	oldDbRoot := filepath.Join(OldBtcdHomeDir(), "db")
	UpgradeDbPathNet(filepath.Join(oldDbRoot, "btcd.db"), "mainnet")
	UpgradeDbPathNet(filepath.Join(oldDbRoot, "btcd_testnet.db"), "testnet")
	UpgradeDbPathNet(filepath.Join(oldDbRoot, "btcd_regtest.db"), "regtest")

	// Remove the old db directory.
	return os.RemoveAll(oldDbRoot)
}

// UpgradeDataPaths moves the application data from its location prior to btcd
// version 0.3.3 to its new location.
func UpgradeDataPaths() error {
	// No need to migrate if the old and new home paths are the same.
	oldHomePath := OldBtcdHomeDir()
	newHomePath := defaultHomeDir
	if oldHomePath == newHomePath {
		return nil
	}

	// Only migrate if the old path exists and the new one doesn't.
	if FileExists(oldHomePath) && !FileExists(newHomePath) {
		// Create the new path.
		PodLog.Infof("Migrating application home path from '%s' to '%s'",
			oldHomePath, newHomePath)
		err := os.MkdirAll(newHomePath, 0700)
		if err != nil {
			return err
		}

		// Move old btcd.conf into new location if needed.
		oldConfPath := filepath.Join(oldHomePath, defaultConfigFilename)
		newConfPath := filepath.Join(newHomePath, defaultConfigFilename)
		if FileExists(oldConfPath) && !FileExists(newConfPath) {
			err := os.Rename(oldConfPath, newConfPath)
			if err != nil {
				return err
			}
		}

		// Move old data directory into new location if needed.
		oldDataPath := filepath.Join(oldHomePath, defaultDataDirname)
		newDataPath := filepath.Join(newHomePath, defaultDataDirname)
		if FileExists(oldDataPath) && !FileExists(newDataPath) {
			err := os.Rename(oldDataPath, newDataPath)
			if err != nil {
				return err
			}
		}

		// Remove the old home if it is empty or show a warning if not.
		ohpEmpty, err := DirEmpty(oldHomePath)
		if err != nil {
			return err
		}
		if ohpEmpty {
			err := os.Remove(oldHomePath)
			if err != nil {
				return err
			}
		} else {
			PodLog.Warnf("Not removing '%s' since it contains files "+
				"not created by this application.  You may "+
				"want to manually move them or delete them.",
				oldHomePath)
		}
	}

	return nil
}

// DoUpgrades performs upgrades to btcd as new versions require it.
func DoUpgrades() error {
	err := UpgradeDbPaths()
	if err != nil {
		return err
	}
	return UpgradeDataPaths()
}
