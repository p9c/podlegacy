package main

import (
	"fmt"
	flags "github.com/jessevdk/go-flags"
	"github.com/parallelcointeam/pod/btcjson"
	"github.com/parallelcointeam/pod/btcutil"
	"github.com/parallelcointeam/pod/fork"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	// unusableFlags are the command usage flags which this utility are not able to use.  In particular it doesn't support websockets and consequently notifications.
	unusableFlags = btcjson.UFWebsocketOnly | btcjson.UFNotification
)

var (
	podHomeDir         = btcutil.AppDataDir("pod", false)
	kopachHomeDir      = btcutil.AppDataDir("kopach", false)
	sacHomeDir         = btcutil.AppDataDir("sac", false)
	defaultConfigFile  = filepath.Join(kopachHomeDir, "kopach.conf")
	defaultRPCServer   = "localhost"
	defaultRPCCertFile = filepath.Join(podHomeDir, "rpc.cert")
)

// config defines the configuration options for podctl. See loadConfig for details on the configuration load process.
type config struct {
	ShowVersion   bool     `short:"V" long:"version" description:"Display version information and exit"`
	ConfigFile    string   `short:"C" long:"configfile" description:"Path to configuration file"`
	RPCUser       string   `short:"u" long:"rpcuser" description:"RPC username"`
	RPCPassword   string   `short:"P" long:"rpcpass" default-mask:"-" description:"RPC password"`
	RPCServer     string   `short:"s" long:"rpcserver" description:"RPC server to connect to"`
	RPCCert       string   `short:"c" long:"rpccert" description:"RPC server certificate chain for validation"`
	TLS           bool     `long:"tls" description:"Enable TLS"`
	Proxy         string   `long:"proxy" description:"Connect via SOCKS5 proxy (eg. 127.0.0.1:9050)"`
	ProxyUser     string   `long:"proxyuser" description:"Username for proxy server"`
	ProxyPass     string   `long:"proxypass" default-mask:"-" description:"Password for proxy server"`
	TestNet3      bool     `long:"testnet" description:"Connect to testnet"`
	SimNet        bool     `long:"simnet" description:"Connect to the simulation test network"`
	TLSSkipVerify bool     `long:"skipverify" description:"Do not verify tls certificates (not recommended!)"`
	Algo          string   `short:"a" long:"algo" description:"Mine with this algorithm, options are blake14lr, cryptonight7v2, keccak, lyra2rev2, scrypt, sha256d, skein, stribog, x11, random or easy"`
	Bench         bool     `long:"bench" description:"Run a benchmark to compare the solution rate for each algorithm on your CPU (stores result in app data directory for 'easy' mining mode to use)"`
	Threads       uint16   `short:"t" long:"threads" description:"Number of threads to spawn (default is -1, meaning all cpu core/threads)"`
	Addresses     []string `short:"A" long:"addr" description:"Addresses to put in block coinbases, chosen randomly from all configured addresses"`
}

// normalizeAddress returns addr with the passed default port appended if there is not already a port specified.
func normalizeAddress(addr string, useTestNet3, useSimNet bool) string {
	_, _, err := net.SplitHostPort(addr)
	if err != nil {
		var defaultPort string
		switch {
		case useTestNet3:
			defaultPort = "21048"
		case useSimNet:
			defaultPort = "41048"
		default:
			defaultPort = "11048"
		}
		return net.JoinHostPort(addr, defaultPort)
	}
	return addr
}

// cleanAndExpandPath expands environement variables and leading ~ in the passed path, cleans the result, and returns it.
func cleanAndExpandPath(path string) string {
	// Expand initial ~ to OS specific home directory.
	if strings.HasPrefix(path, "~") {
		homeDir := filepath.Dir(kopachHomeDir)
		path = strings.Replace(path, "~", homeDir, 1)
	}
	// NOTE: The os.ExpandEnv doesn't work with Windows-style %VARIABLE%, but they variables can still be expanded via POSIX-style $VARIABLE.
	return filepath.Clean(os.ExpandEnv(path))
}

// loadConfig initializes and parses the config using a config file and command line options.
// The configuration proceeds as follows:
// 	1) Start with a default config with sane settings
// 	2) Pre-parse the command line to check for an alternative config file
// 	3) Load configuration file overwriting defaults with any specified options
// 	4) Parse CLI options and overwrite/add any specified options
// The above results in functioning properly without any config settings while still allowing the user to override settings with config files and command line options.  Command line options always take precedence.
func LoadConfig() (*config, []string, error) {
	// Default config.
	cfg := config{
		ConfigFile: defaultConfigFile,
		RPCServer:  defaultRPCServer,
		RPCCert:    defaultRPCCertFile,
	}
	// Pre-parse the command line options to see if an alternative config file, the version flag, or the list commands flag was specified.  Any errors aside from the help message error can be ignored here since they will be caught by the final parse below.
	preCfg := cfg
	preParser := flags.NewParser(&preCfg, flags.HelpFlag)
	_, err := preParser.Parse()
	if err != nil {
		if e, ok := err.(*flags.Error); ok && e.Type == flags.ErrHelp {
			fmt.Fprintln(os.Stderr, err)
			fmt.Fprintln(os.Stderr, "")
			fmt.Fprintln(os.Stderr, "The special parameter `-` "+
				"indicates that a parameter should be read "+
				"from the\nnext unread line from standard "+
				"input.")
			return nil, nil, err
		}
	}
	// Show the version and exit if the version flag was specified.
	appName := filepath.Base(os.Args[0])
	appName = strings.TrimSuffix(appName, filepath.Ext(appName))
	usageMessage := fmt.Sprintf("Use %s -h to show options", appName)
	if preCfg.ShowVersion {
		fmt.Println(appName, "version", version())
		os.Exit(0)
	}
	if _, err := os.Stat(preCfg.ConfigFile); os.IsNotExist(err) {
		// Use config file for RPC server to create default podctl config
		var serverConfigPath string
		serverConfigPath = filepath.Join(podHomeDir, "pod.conf")
		fmt.Println("Creating default config...")
		err := createDefaultConfigFile(preCfg.ConfigFile, serverConfigPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating a default config file: %v\n", err)
		}
	}
	// Load additional config from file.
	parser := flags.NewParser(&cfg, flags.Default)
	err = flags.NewIniParser(parser).ParseFile(preCfg.ConfigFile)
	if err != nil {
		if _, ok := err.(*os.PathError); !ok {
			fmt.Fprintf(os.Stderr, "Error parsing config file: %v\n",
				err)
			fmt.Fprintln(os.Stderr, usageMessage)
			return nil, nil, err
		}
	}
	// Parse command line options again to ensure they take precedence.
	remainingArgs, err := parser.Parse()
	if err != nil {
		if e, ok := err.(*flags.Error); !ok || e.Type != flags.ErrHelp {
			fmt.Fprintln(os.Stderr, usageMessage)
		}
		return nil, nil, err
	}
	// Multiple networks can't be selected simultaneously.
	numNets := 0
	if cfg.TestNet3 {
		numNets++
		fork.IsTestnet = true
	}
	if cfg.SimNet {
		numNets++
	}
	if numNets > 1 {
		str := "%s: The testnet and simnet params can't be used " +
			"together -- choose one of the two"
		err := fmt.Errorf(str, "loadConfig")
		fmt.Fprintln(os.Stderr, err)
		return nil, nil, err
	}

	// Handle environment variable expansion in the RPC certificate path.
	cfg.RPCCert = cleanAndExpandPath(cfg.RPCCert)
	// Add default port to RPC server based on --testnet and --wallet flags if needed.
	cfg.RPCServer = normalizeAddress(cfg.RPCServer, cfg.TestNet3,
		cfg.SimNet)
	return &cfg, remainingArgs, nil
}

// createDefaultConfig creates a basic config file at the given destination path. For this it tries to read the config file for the RPC server (either pod or sac), and extract the RPC user and password from it.
func createDefaultConfigFile(destinationPath, serverConfigPath string) error {
	// Read the RPC server config
	serverConfigFile, err := os.Open(serverConfigPath)
	if err != nil {
		return err
	}
	defer serverConfigFile.Close()
	content, err := ioutil.ReadAll(serverConfigFile)
	if err != nil {
		return err
	}
	// content := []byte(samplePodCtlConf)
	// Extract the rpcuser
	rpcUserRegexp, err := regexp.Compile(`(?m)^\s*rpcuser=([^\s]+)`)
	if err != nil {
		return err
	}
	userSubmatches := rpcUserRegexp.FindSubmatch(content)
	if userSubmatches == nil {
		// No user found, nothing to do
		return nil
	}
	// Extract the rpcpass
	rpcPassRegexp, err := regexp.Compile(`(?m)^\s*rpcpass=([^\s]+)`)
	if err != nil {
		return err
	}
	passSubmatches := rpcPassRegexp.FindSubmatch(content)
	if passSubmatches == nil {
		// No password found, nothing to do
		return nil
	}
	// Extract the TLS
	TLSRegexp, err := regexp.Compile(`(?m)^\s*TLS=(0|1)(?:\s|$)`)
	if err != nil {
		return err
	}
	TLSSubmatches := TLSRegexp.FindSubmatch(content)
	// Create the destination directory if it does not exists
	err = os.MkdirAll(filepath.Dir(destinationPath), 0700)
	if err != nil {
		return err
	}
	// Create the destination file and write the rpcuser and rpcpass to it
	dest, err := os.OpenFile(destinationPath,
		os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		fmt.Println("ERROR", err)
		return err
	}
	defer dest.Close()
	destString := fmt.Sprintf("rpcuser=%s\nrpcpass=%s\n",
		string(userSubmatches[1]), string(passSubmatches[1]))
	if TLSSubmatches != nil {
		destString += fmt.Sprintf("TLS=%s\n", TLSSubmatches[1])
	}
	output := ";;; Defaults created from local pod configuration:\n" + destString + "\n" + sampleKopachConf
	dest.WriteString(output)
	return nil
}
