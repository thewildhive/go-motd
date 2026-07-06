package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	"motd/config"
	"motd/display"
	"motd/media"
	"motd/system"
	"motd/update"
)

var (
	VERSION   = "dev"
	BUILDDATE = "unknown"
)

const curlTimeout = 5 * time.Second

func main() {
	if handleSubcommand() {
		return
	}

	showHelp := flag.Bool("h", false, "Show help message")
	showVersion := flag.Bool("v", false, "Show version information")
	debug := flag.Bool("d", false, "Enable debug mode")
	configPath := flag.String("config", "", "Load config from a specific JSON file")
	noConfig := flag.Bool("no-config", false, "Skip config loading and show system information only")
	jsonOutput := flag.Bool("json", false, "Output machine-readable JSON")
	noColor := flag.Bool("no-color", false, "Disable ANSI colors")
	servicesFilter := flag.String("services", "", "Only show selected media services (comma-separated)")
	flag.Parse()

	if *noColor || *jsonOutput || os.Getenv("NO_COLOR") != "" {
		display.SetColorEnabled(false)
	}

	if *showHelp {
		usage()
		return
	}

	if *showVersion {
		fmt.Printf("MOTD Script v%s (Built %s)\n", VERSION, BUILDDATE)
		return
	}

	client := &http.Client{
		Timeout: curlTimeout,
		Transport: &http.Transport{
			MaxIdleConns:       10,
			IdleConnTimeout:    30 * time.Second,
			DisableCompression: false,
		},
	}

	cfg, err := config.Load(*configPath, *noConfig, func(msg string, args ...interface{}) {
		display.DebugLog(*debug, msg, args...)
	})
	if err != nil {
		if legacyErr, ok := err.(*config.LegacyConfigError); ok {
			config.PrintLegacyConfigError(legacyErr)
			os.Exit(1)
		}
		fmt.Printf("%sError loading configuration: %v%s\n", display.Red, err, display.Reset)
		os.Exit(1)
	}

	if *noConfig {
		display.DebugLog(*debug, "Using system-only defaults")
	} else {
		display.DebugLog(*debug, "Runtime configuration ready")
	}

	serviceSet, err := parseServiceFilter(*servicesFilter)
	if err != nil {
		fmt.Printf("%sError: %v%s\n", display.Red, err, display.Reset)
		os.Exit(1)
	}

	if *jsonOutput {
		renderJSON(cfg, serviceSet, client, *debug)
		return
	}

	display.PrintHeader()

	if msg := update.CheckUpdate(VERSION, client); msg != "" {
		fmt.Printf("%s⚠ %s%s\n\n", display.Yellow, msg, display.Reset)
	}

	display.PrintSection("System Information")

	sysCfg := system.ConfigAccessorFrom(cfg)
	showPlatformSystemInfo(sysCfg, *debug)

	display.PrintSection("Services & Resources")

	system.ShowDocker(*debug)
	system.ShowCompose(sysCfg, *debug)
	system.ShowProcesses(sysCfg, *debug)
	system.ShowUser(sysCfg, *debug)
	system.ShowDisk(sysCfg, *debug)
	system.ShowTemp(sysCfg, *debug)
	media.ShowMediaServices(cfg, serviceSet, client, *debug)

	fmt.Println()
}

func handleSubcommand() bool {
	if len(os.Args) < 2 {
		return false
	}
	switch os.Args[1] {
	case "self-update":
		client := &http.Client{Timeout: curlTimeout}
		update.HandleSelfUpdate(VERSION, client)
		return true
	case "configure":
		handleConfigure()
		return true
	case "check-config":
		handleCheckConfig(os.Args[2:])
		return true
	default:
		return false
	}
}

func showPlatformSystemInfo(cfg system.ConfigAccessor, debug bool) {
	system.ShowOS(cfg, debug)
	system.ShowUptime(cfg, debug)
	system.ShowLoad(cfg, debug)
	system.ShowMemory(cfg, debug)
	system.ShowBandwidth(cfg, debug)
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
  -json           Output machine-readable JSON
  -no-color       Disable ANSI colors
  -services LIST  Only show selected media services (comma-separated)

Commands:
  self-update     Update to the latest version from GitHub releases
  configure       Create or edit the config file
  check-config    Validate configuration and print diagnostics

Configuration Files:
  Optional; only required for media integrations or custom system paths.
  ~/.config/motd/config.json   (highest priority)
  /opt/motd/config.json        (fallback)`)
}
