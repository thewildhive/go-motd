package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"
)

var VERSION = "dev"

const CURL_TIMEOUT = 5 * time.Second

var (
	config     Config
	httpClient *http.Client
	debugMode  bool
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "self-update" {
		handleSelfUpdate()
		return
	}

	showHelp := flag.Bool("h", false, "Show help message")
	showVersion := flag.Bool("v", false, "Show version information")
	debug := flag.Bool("d", false, "Enable debug mode")
	configPath := flag.String("config", "", "Load config from a specific JSON file")
	noConfig := flag.Bool("no-config", false, "Skip config loading and show system information only")
	flag.Parse()

	if *showHelp {
		usage()
		return
	}

	if *showVersion {
		fmt.Printf("MOTD Script v%s\n", VERSION)
		return
	}

	debugMode = *debug
	httpClient = &http.Client{
		Timeout: CURL_TIMEOUT,
		Transport: &http.Transport{
			MaxIdleConns:       10,
			IdleConnTimeout:    30 * time.Second,
			DisableCompression: false,
		},
	}

	loadConfig(*configPath, *noConfig)

	printHeader()
	printSection("System Information")

	showOS()
	showUptime()
	showLoad()
	showMemory()
	showBandwidth()

	printSection("Services & Resources")

	showUser()
	showProcesses()
	showDocker()
	showDisk()
	showTemp()

	showMediaServices()

	fmt.Println()
}

func usage() {
	fmt.Println(`Usage: motd [OPTIONS]
Display Message of the Day (MOTD) with system and media service statistics.

Options:
  -h              Show this help message
  -v              Show version information
  -d              Enable debug mode
  -config PATH    Load config from a specific JSON file
  -no-config      Skip config loading and show system information only

Commands:
  self-update     Update to the latest version from GitHub releases

Configuration Files:
  Optional; only required for media integrations or custom system paths.
  ~/.config/motd/config.json   (highest priority)
  /opt/motd/config.json        (fallback)`)
}
