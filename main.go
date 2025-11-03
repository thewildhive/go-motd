package main

import (
	"crypto/sha256"
	"encoding/json"
	"encoding/xml"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	VERSION         = "0.2.5"
	CURL_TIMEOUT    = 5 * time.Second
	DOT_LABEL_WIDTH = 22
)

// ANSI Colors
const (
	RED    = "\033[0;31m"
	GREEN  = "\033[0;32m"
	YELLOW = "\033[0;33m"
	BLUE   = "\033[0;34m"
	CYAN   = "\033[0;36m"
	BOLD   = "\033[1m"
	RESET  = "\033[0m"
)

// ServiceConfig holds configuration for a single service instance
type ServiceConfig struct {
	Name    string `yaml:"name"`
	URL     string `yaml:"url"`
	APIKey  string `yaml:"api_key,omitempty"`
	Token   string `yaml:"token,omitempty"`
	Enabled bool   `yaml:"enabled"`
}

// Config holds application configuration
type Config struct {
	Services struct {
		Plex     []ServiceConfig `yaml:"plex"`
		Jellyfin []ServiceConfig `yaml:"jellyfin"`
		Sonarr   []ServiceConfig `yaml:"sonarr"`
		Radarr   []ServiceConfig `yaml:"radarr"`
		Organizr []ServiceConfig `yaml:"organizr"`
	} `yaml:"services"`
	System struct {
		ComposeDir string `yaml:"compose_dir"`
		TankMount  string `yaml:"tank_mount"`
		Network    struct {
			Interface string `yaml:"interface,omitempty"`
		} `yaml:"network,omitempty"`
	} `yaml:"system"`
}

// Global state
var (
	config     Config
	httpClient *http.Client
	debugMode  bool
)

// Config file paths in priority order
var configPaths = []string{
	filepath.Join(getUserHome(), ".config", "motd", "config.yml"),
	"/opt/motd/config.yml",
}

func main() {
	// Handle self-update subcommand first (before flag parsing)
	if len(os.Args) > 1 && os.Args[1] == "self-update" {
		handleSelfUpdate()
		return
	}

	showHelp := flag.Bool("h", false, "Show help message")
	showVersion := flag.Bool("v", false, "Show version information")
	debug := flag.Bool("d", false, "Enable debug mode")
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

	// Initialize HTTP client
	httpClient = &http.Client{
		Timeout: CURL_TIMEOUT,
		Transport: &http.Transport{
			MaxIdleConns:       10,
			IdleConnTimeout:    30 * time.Second,
			DisableCompression: false,
		},
	}

	// Load configuration
	loadConfig()

	// Display MOTD
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

	// Check if any media service is configured
	if hasMediaServices() {
		printSection("Media Services")
		showPlex()
		showJellyfin()
		showSonarr()
		showRadarr()
		showOrganizr()
	}

	fmt.Println()
}

func usage() {
	fmt.Println(`Usage: motd [OPTIONS]
Display Message of the Day (MOTD) with system and media service statistics.

Options:
  -h              Show this help message
  -v              Show version information
  -d              Enable debug mode

Commands:
  self-update     Update to the latest version from GitHub releases

Configuration Files:
  ~/.config/motd/config.yml    (highest priority)
  /opt/motd/config.yml         (fallback)`)
}

func debugLog(msg string, args ...interface{}) {
	if debugMode {
		fmt.Fprintf(os.Stderr, "[DEBUG] "+msg+"\n", args...)
	}
}

func loadYAMLConfig() (Config, error) {
	var yamlConfig Config

	for _, configPath := range configPaths {
		if _, err := os.Stat(configPath); err == nil {
			debugLog("Loading YAML config from: %s", configPath)

			data, err := os.ReadFile(configPath)
			if err != nil {
				debugLog("Failed to read config file %s: %v", configPath, err)
				continue
			}

			if err := yaml.Unmarshal(data, &yamlConfig); err != nil {
				debugLog("Failed to parse YAML config %s: %v", configPath, err)
				continue
			}

			debugLog("Successfully loaded YAML config from: %s", configPath)
			return yamlConfig, nil
		}
	}

	debugLog("No YAML config files found")
	return yamlConfig, fmt.Errorf("no YAML config files found")
}

func loadConfig() {
	// Load YAML configuration
	yamlConfig, err := loadYAMLConfig()
	if err != nil {
		fmt.Printf("%sError: No configuration file found. Please create a config file at:%s\n", RED, RESET)
		fmt.Printf("  %s\n", configPaths[0])
		fmt.Printf("  %s\n", configPaths[1])
		os.Exit(1)
	}

	config = yamlConfig
	debugLog("Using YAML configuration")
}

func hasMediaServices() bool {
	// Check if any service instances are configured and enabled
	for _, plex := range config.Services.Plex {
		if plex.Enabled && (plex.Token != "") {
			return true
		}
	}

	for _, jellyfin := range config.Services.Jellyfin {
		if jellyfin.Enabled && (jellyfin.Token != "") {
			return true
		}
	}

	for _, sonarr := range config.Services.Sonarr {
		if sonarr.Enabled && (sonarr.APIKey != "") {
			return true
		}
	}

	for _, radarr := range config.Services.Radarr {
		if radarr.Enabled && (radarr.APIKey != "") {
			return true
		}
	}

	for _, organizr := range config.Services.Organizr {
		if organizr.Enabled && (organizr.APIKey != "") {
			return true
		}
	}

	return false
}

func dotLabel(label string) {
	fmt.Print(label)
	dots := DOT_LABEL_WIDTH - len(label)
	if dots > 0 {
		fmt.Print(strings.Repeat(".", dots))
	}
	fmt.Print(": ")
}

func printHeader() {
	fmt.Println()

	hostname, _ := os.Hostname()

	// Try figlet + lolcat
	if hasFiglet() && hasLolcat() {
		cmd := exec.Command("sh", "-c", fmt.Sprintf("figlet '%s' | lolcat -f", hostname))
		cmd.Stdout = os.Stdout
		cmd.Run()
	} else {
		// Dynamic box sizing based on hostname length
		label := "Connected to: "
		contentLength := len(label) + len(hostname)

		// Total line width: ║ + 2 spaces + content + 1 space + ║ = content + 5
		totalWidth := contentLength + 5
		if totalWidth < 40 {
			totalWidth = 40 // Minimum total width (matches original 38 chars of ═)
		}

		// Border has 2 fewer chars than total (for corner pieces)
		borderWidth := totalWidth - 2

		// Build borders
		topBorder := "╔" + strings.Repeat("═", borderWidth) + "╗"
		bottomBorder := "╚" + strings.Repeat("═", borderWidth) + "╝"

		// Content width is total minus the border chars: ║ + 2 spaces + 1 space + ║ = 5
		paddedContent := fmt.Sprintf("%-*s", totalWidth-5, label+hostname)

		fmt.Printf("%s%s%s%s\n", BOLD, CYAN, topBorder, RESET)
		fmt.Printf("%s%s║  %s ║%s\n", BOLD, CYAN, paddedContent, RESET)
		fmt.Printf("%s%s%s%s\n", BOLD, CYAN, bottomBorder, RESET)
	}
	fmt.Println()
}

func printSection(title string) {
	fmt.Printf("\n%s%s━━━ %s ━━━%s\n", BOLD, CYAN, title, RESET)
}

func showOS() {
	release := "Unknown"
	data, err := os.ReadFile("/etc/os-release")
	if err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(line, "PRETTY_NAME=") {
				release = strings.Trim(strings.TrimPrefix(line, "PRETTY_NAME="), "\"")
				break
			}
		}
	}
	dotLabel("OS Release")
	fmt.Printf("%s%s%s\n", BLUE, release, RESET)
}

func showUptime() {
	output, err := exec.Command("uptime", "-p").Output()
	uptime := "unknown"
	if err == nil {
		uptime = strings.TrimPrefix(strings.TrimSpace(string(output)), "up ")
	}
	dotLabel("Uptime")
	fmt.Printf("%s%s%s\n", BLUE, uptime, RESET)
}

func showLoad() {
	output, err := exec.Command("uptime").Output()
	load := ""
	if err == nil {
		parts := strings.Split(string(output), "load average: ")
		if len(parts) > 1 {
			load = strings.TrimSpace(parts[1])
		}
	}
	dotLabel("CPU Load")
	fmt.Printf("%s%s%s\n", BLUE, load, RESET)
}

func showMemory() {
	output, err := exec.Command("free", "-b").Output()
	if err != nil {
		return
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "Mem:") {
			fields := strings.Fields(line)
			if len(fields) >= 3 {
				total, _ := strconv.ParseFloat(fields[1], 64)
				used, _ := strconv.ParseFloat(fields[2], 64)
				totalGB := total / 1073741824.0
				usedGB := used / 1073741824.0

				dotLabel("Memory")
				fmt.Printf("%s%.2f GB / %.2f GB%s\n", BLUE, usedGB, totalGB, RESET)
			}
			break
		}
	}
}

func showBandwidth() {
	if !hasCommand("vnstat") {
		return
	}

	// Determine which interface to use
	interfaceName := config.System.Network.Interface
	if interfaceName == "" {
		interfaceName = "enp7s0" // fallback as specified in requirements
	}

	// Always use monthly data with 30-day estimates (hardcoded)
	output, err := exec.Command("vnstat", "--json", "m", interfaceName).Output()
	if err != nil {
		return
	}

	var data struct {
		Interfaces []struct {
			ID      string `json:"id"`
			Traffic struct {
				Month []struct {
					Rx   uint64 `json:"rx"`
					Tx   uint64 `json:"tx"`
					Date struct {
						Year  int `json:"year"`
						Month int `json:"month"`
					} `json:"date"`
				} `json:"month"`
			} `json:"traffic"`
		} `json:"interfaces"`
	}

	if err := json.Unmarshal(output, &data); err != nil {
		return
	}

	if len(data.Interfaces) == 0 || len(data.Interfaces[0].Traffic.Month) == 0 {
		return
	}

	// Take the latest month entry
	month := data.Interfaces[0].Traffic.Month[len(data.Interfaces[0].Traffic.Month)-1]

	rxGB := float64(month.Rx) / 1073741824.0
	txGB := float64(month.Tx) / 1073741824.0

	// Estimate for 30‑day month (always using monthly data)
	day := float64(time.Now().Day())
	rxEst := rxGB * (30.0 / day)
	txEst := txGB * (30.0 / day)

	dotLabel("Bandwidth (rx)")
	fmt.Printf("%s%.2f GB / %.2f GB est%s\n", BLUE, rxGB, rxEst, RESET)
	dotLabel("Bandwidth (tx)")
	fmt.Printf("%s%.2f GB / %.2f GB est%s\n", BLUE, txGB, txEst, RESET)
}

func showUser() {
	output, err := exec.Command("sh", "-c", "who | awk '{print $1}' | sort -u | wc -l").Output()
	if err != nil {
		return
	}

	count := strings.TrimSpace(string(output))
	dotLabel("Logged in users")
	fmt.Printf("%s%s%s\n", BLUE, count, RESET)
}

func showProcesses() {
	output, err := exec.Command("sh", "-c", "ps -e --no-headers | wc -l").Output()
	if err != nil {
		return
	}

	count := strings.TrimSpace(string(output))
	dotLabel("Processes")
	fmt.Printf("%s%s%s\n", BLUE, count, RESET)
}

func showDocker() {
	if !hasCommand("docker") {
		return
	}

	output, err := exec.Command("docker", "ps", "-q").Output()
	if err != nil {
		return
	}

	count := len(strings.Split(strings.TrimSpace(string(output)), "\n"))
	if strings.TrimSpace(string(output)) == "" {
		count = 0
	}

	dotLabel("Docker Containers")
	fmt.Printf("%s%d running%s\n", BLUE, count, RESET)
}

func showDisk() {
	// Root disk
	output, err := exec.Command("df", "/").Output()
	if err == nil {
		lines := strings.Split(string(output), "\n")
		if len(lines) >= 2 {
			fields := strings.Fields(lines[1])
			if len(fields) >= 5 {
				used := fields[2]
				total := fields[1]
				pct := strings.TrimSuffix(fields[4], "%")

				usedVal, _ := strconv.ParseFloat(used, 64)
				totalVal, _ := strconv.ParseFloat(total, 64)

				usedGB := usedVal / 1048576.0
				totalGB := totalVal / 1048576.0

				dotLabel("Disk (/)")
				fmt.Printf("%s%.2f GB / %.2f GB (%s%% used)%s\n", BLUE, usedGB, totalGB, pct, RESET)
			}
		}
	}

	// Tank disk
	if config.System.TankMount != "" {
		output, err := exec.Command("df", config.System.TankMount).Output()
		if err == nil {
			lines := strings.Split(string(output), "\n")
			if len(lines) >= 2 {
				fields := strings.Fields(lines[1])
				if len(fields) >= 5 {
					used := fields[2]
					total := fields[1]
					pct := strings.TrimSuffix(fields[4], "%")

					usedVal, _ := strconv.ParseFloat(used, 64)
					totalVal, _ := strconv.ParseFloat(total, 64)

					usedGB := usedVal / 1048576.0
					totalGB := totalVal / 1048576.0

					dotLabel(fmt.Sprintf("Disk (%s)", config.System.TankMount))
					fmt.Printf("%s%.2f GB / %.2f GB (%s%% used)%s\n", BLUE, usedGB, totalGB, pct, RESET)
				}
			}
		}
	}
}

func showTemp() {
	if !hasCommand("sensors") {
		return
	}

	output, err := exec.Command("sensors").Output()
	if err != nil {
		return
	}

	for _, line := range strings.Split(string(output), "\n") {
		if strings.Contains(line, "Package id 0:") {
			fields := strings.Fields(line)
			if len(fields) >= 4 {
				temp := fields[3]
				dotLabel("CPU Temperature")
				fmt.Printf("%s%s%s\n", RED, temp, RESET)
				break
			}
		}
	}
}

func showPlex() {
	if len(config.Services.Plex) == 0 {
		return
	}

	// Parse XML structure
	type MediaContainer struct {
		Size   int `xml:"size,attr"`
		Videos []struct {
			TranscodeSession struct {
				VideoDecision string `xml:"videoDecision,attr"`
			} `xml:"TranscodeSession"`
			Session struct {
				Bandwidth int `xml:"bandwidth,attr"`
			} `xml:"Session"`
		} `xml:"Video"`
	}

	for _, plex := range config.Services.Plex {
		if !plex.Enabled || plex.Token == "" {
			continue
		}

		url := fmt.Sprintf("%s/status/sessions?X-Plex-Token=%s", plex.URL, plex.Token)
		resp, err := httpClient.Get(url)
		if err != nil {
			debugLog("Plex request failed for %s: %v", plex.Name, err)
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			debugLog("Plex returned status %d for %s", resp.StatusCode, plex.Name)
			continue
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			debugLog("Failed to read Plex response for %s: %v", plex.Name, err)
			continue
		}

		var container MediaContainer
		if err := xml.Unmarshal(body, &container); err != nil {
			debugLog("Failed to parse Plex XML for %s: %v", plex.Name, err)
			continue
		}

		count := container.Size
		tcount := 0
		bw := 0

		for _, video := range container.Videos {
			if video.TranscodeSession.VideoDecision == "transcode" {
				tcount++
			}
			bw += video.Session.Bandwidth
		}

		bwMbps := float64(bw) / 1000.0

		// Display with instance name
		label := "Plex"
		if plex.Name != "Default" {
			label = fmt.Sprintf("Plex (%s)", plex.Name)
		}

		dotLabel(label)
		if count == 0 {
			fmt.Printf("%sNo active streams%s\n", GREEN, RESET)
		} else if tcount == 0 {
			fmt.Printf("%s%d streams (%.2f Mbps)%s\n", YELLOW, count, bwMbps, RESET)
		} else {
			fmt.Printf("%s%d streams, %d transcodes (%.2f Mbps)%s\n", RED, count, tcount, bwMbps, RESET)
		}
	}
}

func showJellyfin() {
	if len(config.Services.Jellyfin) == 0 {
		return
	}

	for _, jellyfin := range config.Services.Jellyfin {
		if !jellyfin.Enabled || jellyfin.Token == "" {
			continue
		}

		req, err := http.NewRequest("GET", jellyfin.URL+"/Sessions", nil)
		if err != nil {
			debugLog("Jellyfin request failed for %s: %v", jellyfin.Name, err)
			continue
		}
		req.Header.Set("X-Emby-Token", jellyfin.Token)

		resp, err := httpClient.Do(req)
		if err != nil {
			debugLog("Jellyfin request failed for %s: %v", jellyfin.Name, err)
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			debugLog("Jellyfin returned status %d for %s", resp.StatusCode, jellyfin.Name)
			continue
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			debugLog("Failed to read Jellyfin response for %s: %v", jellyfin.Name, err)
			continue
		}

		var sessions []map[string]interface{}
		if err := json.Unmarshal(body, &sessions); err != nil {
			debugLog("Failed to parse Jellyfin JSON for %s: %v", jellyfin.Name, err)
			continue
		}

		count := 0
		tcount := 0
		bw := 0.0

		for _, session := range sessions {
			if session["NowPlayingItem"] != nil {
				count++
				if playState, ok := session["PlayState"].(map[string]interface{}); ok {
					if playMethod, ok := playState["PlayMethod"].(string); ok && playMethod == "Transcode" {
						tcount++
					}
				}
			}
		}

		// Display with instance name
		label := "Jellyfin"
		if jellyfin.Name != "Default" {
			label = fmt.Sprintf("Jellyfin (%s)", jellyfin.Name)
		}

		dotLabel(label)
		if count == 0 {
			fmt.Printf("%sNo active streams%s\n", GREEN, RESET)
		} else if tcount == 0 {
			fmt.Printf("%s%d streams (%.2f Mbps)%s\n", YELLOW, count, bw, RESET)
		} else {
			fmt.Printf("%s%d streams, %d transcodes (%.2f Mbps)%s\n", RED, count, tcount, bw, RESET)
		}
	}
}

func showSonarr() {
	if len(config.Services.Sonarr) == 0 {
		return
	}

	for _, sonarr := range config.Services.Sonarr {
		if !sonarr.Enabled || sonarr.APIKey == "" {
			continue
		}

		url := fmt.Sprintf("%s/api/v3/wanted/missing?apikey=%s", sonarr.URL, sonarr.APIKey)
		resp, err := httpClient.Get(url)
		if err != nil {
			debugLog("Sonarr request failed for %s: %v", sonarr.Name, err)
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			debugLog("Sonarr returned status %d for %s", resp.StatusCode, sonarr.Name)
			continue
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			debugLog("Failed to read Sonarr response for %s: %v", sonarr.Name, err)
			continue
		}

		var result struct {
			Records []interface{} `json:"records"`
		}

		if err := json.Unmarshal(body, &result); err != nil {
			debugLog("Failed to parse Sonarr JSON for %s: %v", sonarr.Name, err)
			continue
		}

		count := len(result.Records)

		// Display with instance name
		label := "Sonarr"
		if sonarr.Name != "Default" {
			label = fmt.Sprintf("Sonarr (%s)", sonarr.Name)
		}

		dotLabel(label)
		if count == 0 {
			fmt.Printf("%sNo missing episodes%s\n", GREEN, RESET)
		} else {
			plural := ""
			if count != 1 {
				plural = "s"
			}
			fmt.Printf("%s%d missing episode%s%s\n", YELLOW, count, plural, RESET)
		}
	}
}

func showRadarr() {
	if len(config.Services.Radarr) == 0 {
		return
	}

	for _, radarr := range config.Services.Radarr {
		if !radarr.Enabled || radarr.APIKey == "" {
			continue
		}

		url := fmt.Sprintf("%s/api/v3/wanted/missing?apikey=%s", radarr.URL, radarr.APIKey)
		resp, err := httpClient.Get(url)
		if err != nil {
			debugLog("Radarr request failed for %s: %v", radarr.Name, err)
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			debugLog("Radarr returned status %d for %s", resp.StatusCode, radarr.Name)
			continue
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			debugLog("Failed to read Radarr response for %s: %v", radarr.Name, err)
			continue
		}

		var result struct {
			Records []map[string]interface{} `json:"records"`
		}

		if err := json.Unmarshal(body, &result); err != nil {
			debugLog("Failed to parse Radarr JSON for %s: %v", radarr.Name, err)
			continue
		}

		count := 0
		for _, record := range result.Records {
			if isAvail, ok := record["isAvailable"].(bool); ok && isAvail {
				count++
			}
		}

		// Display with instance name
		label := "Radarr"
		if radarr.Name != "Default" {
			label = fmt.Sprintf("Radarr (%s)", radarr.Name)
		}

		dotLabel(label)
		if count == 0 {
			fmt.Printf("%sNo missing movies%s\n", GREEN, RESET)
		} else {
			plural := ""
			if count != 1 {
				plural = "s"
			}
			fmt.Printf("%s%d missing movie%s%s\n", YELLOW, count, plural, RESET)
		}
	}
}

func showOrganizr() {
	if len(config.Services.Organizr) == 0 {
		return
	}

	for _, organizr := range config.Services.Organizr {
		if !organizr.Enabled || organizr.APIKey == "" {
			continue
		}

		req, err := http.NewRequest("GET", organizr.URL+"/api/v2/requests", nil)
		if err != nil {
			debugLog("Organizr request failed for %s: %v", organizr.Name, err)
			continue
		}
		req.Header.Set("Authorization", "Bearer "+organizr.APIKey)

		resp, err := httpClient.Do(req)
		if err != nil {
			debugLog("Organizr request failed for %s: %v", organizr.Name, err)
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			debugLog("Organizr returned status %d for %s", resp.StatusCode, organizr.Name)
			continue
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			debugLog("Failed to read Organizr response for %s: %v", organizr.Name, err)
			continue
		}

		var result struct {
			Data struct {
				Total int `json:"total"`
			} `json:"data"`
		}

		if err := json.Unmarshal(body, &result); err != nil {
			debugLog("Failed to parse Organizr JSON for %s: %v", organizr.Name, err)
			continue
		}

		count := result.Data.Total

		// Display with instance name
		label := "Organizr"
		if organizr.Name != "Default" {
			label = fmt.Sprintf("Organizr (%s)", organizr.Name)
		}

		dotLabel(label)
		if count == 0 {
			fmt.Printf("%sNo requests%s\n", GREEN, RESET)
		} else {
			plural := ""
			if count != 1 {
				plural = "s"
			}
			fmt.Printf("%s%d request%s%s\n", YELLOW, count, plural, RESET)
		}
	}
}

// Helper functions
func getUserHome() string {
	usr, err := user.Current()
	if err != nil {
		// Fallback to HOME environment variable if user.Current() fails
		return os.Getenv("HOME")
	}
	return usr.HomeDir
}

func hasCommand(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}

func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}

func hasFiglet() bool {
	return hasCommand("figlet")
}

func hasLolcat() bool {
	return hasCommand("lolcat")
}

// getDefaultInterface attempts to detect the default network interface
func getDefaultInterface() string {
	// Try multiple methods to find the default interface

	// Method 1: Use ip route to find default route
	if output, err := exec.Command("ip", "route", "show", "default").Output(); err == nil {
		lines := strings.Split(strings.TrimSpace(string(output)), "\n")
		for _, line := range lines {
			if strings.Contains(line, "default via") {
				fields := strings.Fields(line)
				for i, field := range fields {
					if field == "dev" && i+1 < len(fields) {
						return fields[i+1]
					}
				}
			}
		}
	}

	// Method 2: Use route command (older systems)
	if output, err := exec.Command("route", "-n").Output(); err == nil {
		lines := strings.Split(strings.TrimSpace(string(output)), "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "0.0.0.0") || strings.HasPrefix(line, "default") {
				fields := strings.Fields(line)
				if len(fields) >= 8 {
					return fields[7] // Interface name is usually the 8th field
				}
			}
		}
	}

	// Method 3: Fall back to common interface names
	commonInterfaces := []string{"eth0", "enp0s3", "ens33", "en0", "wlan0", "wlp2s0"}
	for _, iface := range commonInterfaces {
		if output, err := exec.Command("ip", "link", "show", iface).Output(); err == nil {
			if strings.Contains(string(output), "state UP") {
				return iface
			}
		}
	}

	// Method 4: Get first non-loopback interface
	if output, err := exec.Command("ip", "link", "show").Output(); err == nil {
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			if strings.Contains(line, "state UP") && !strings.Contains(line, "lo:") {
				fields := strings.Fields(line)
				if len(fields) >= 2 {
					ifaceName := strings.TrimSuffix(fields[1], ":")
					if ifaceName != "lo" {
						return ifaceName
					}
				}
			}
		}
	}

	return ""
}

// Self-update functionality
type GitHubRelease struct {
	TagName string `json:"tag_name"`
	Name    string `json:"name"`
	Assets  []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
	} `json:"assets"`
}

func handleSelfUpdate() {
	force := false
	if len(os.Args) > 2 && os.Args[2] == "--force" {
		force = true
	}

	// Initialize HTTP client (normally done in main())
	httpClient = &http.Client{
		Timeout: CURL_TIMEOUT,
		Transport: &http.Transport{
			MaxIdleConns:       10,
			IdleConnTimeout:    30 * time.Second,
			DisableCompression: false,
		},
	}

	fmt.Printf("%sChecking for updates...%s\n", CYAN, RESET)

	// Get latest release info
	release, err := getLatestRelease()
	if err != nil {
		fmt.Printf("%sError checking for updates: %v%s\n", RED, err, RESET)
		os.Exit(1)
	}

	latestVersion := strings.TrimPrefix(release.TagName, "v")
	currentVersion := VERSION

	fmt.Printf("Current version: %s%s%s\n", BLUE, currentVersion, RESET)
	fmt.Printf("Latest version:  %s%s%s\n", BLUE, latestVersion, RESET)

	// Compare versions
	if !force && compareVersions(currentVersion, latestVersion) >= 0 {
		fmt.Printf("%sAlready running the latest version!%s\n", GREEN, RESET)
		return
	}

	if !force {
		fmt.Printf("\n%sA new version is available!%s\n", YELLOW, RESET)
		fmt.Printf("Update from %s%s%s to %s%s%s?\n", RED, currentVersion, RESET, GREEN, latestVersion, RESET)
		fmt.Print("Update? [y/N]: ")

		var response string
		fmt.Scanln(&response)
		if strings.ToLower(response) != "y" && strings.ToLower(response) != "yes" {
			fmt.Println("Update cancelled.")
			return
		}
	}

	fmt.Printf("\n%sUpdating to version %s...%s\n", CYAN, latestVersion, RESET)

	// Perform update
	if err := performUpdate(release, latestVersion); err != nil {
		fmt.Printf("%sUpdate failed: %v%s\n", RED, err, RESET)
		os.Exit(1)
	}

	fmt.Printf("%sSuccessfully updated to version %s!%s\n", GREEN, latestVersion, RESET)
}

func getLatestRelease() (*GitHubRelease, error) {
	url := "https://api.github.com/repos/thewildhive/go-motd/releases/latest"

	resp, err := httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch release info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var release GitHubRelease
	if err := json.Unmarshal(body, &release); err != nil {
		return nil, fmt.Errorf("failed to parse release info: %w", err)
	}

	return &release, nil
}

func compareVersions(current, latest string) int {
	currentParts := strings.Split(current, ".")
	latestParts := strings.Split(latest, ".")

	maxLen := len(currentParts)
	if len(latestParts) > maxLen {
		maxLen = len(latestParts)
	}

	for i := 0; i < maxLen; i++ {
		var currentNum, latestNum int

		if i < len(currentParts) {
			currentNum, _ = strconv.Atoi(currentParts[i])
		}
		if i < len(latestParts) {
			latestNum, _ = strconv.Atoi(latestParts[i])
		}

		if currentNum < latestNum {
			return -1
		} else if currentNum > latestNum {
			return 1
		}
	}

	return 0
}

func performUpdate(release *GitHubRelease, version string) error {
	// Determine platform and asset name
	assetName := getPlatformAssetName()
	if assetName == "" {
		return fmt.Errorf("unsupported platform: %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	// Find the asset URL
	var downloadURL string
	for _, asset := range release.Assets {
		if strings.Contains(asset.Name, assetName) {
			downloadURL = asset.URL
			break
		}
	}

	if downloadURL == "" {
		return fmt.Errorf("could not find release asset for platform %s", assetName)
	}

	// Get checksums
	checksums, err := getChecksums(release)
	if err != nil {
		return fmt.Errorf("failed to get checksums: %w", err)
	}

	// Download binary
	tempFile, err := downloadBinary(downloadURL, assetName, checksums)
	if err != nil {
		return fmt.Errorf("failed to download binary: %w", err)
	}
	defer os.Remove(tempFile)

	// Get current executable path
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	// Create backup
	backupPath := execPath + ".backup"
	if err := copyFile(execPath, backupPath); err != nil {
		return fmt.Errorf("failed to create backup: %w", err)
	}
	defer os.Remove(backupPath)

	// Replace binary
	if err := replaceBinary(tempFile, execPath); err != nil {
		// Attempt rollback
		if rollbackErr := copyFile(backupPath, execPath); rollbackErr != nil {
			return fmt.Errorf("update failed and rollback failed: %v (rollback error: %v)", err, rollbackErr)
		}
		return fmt.Errorf("update failed, rolled back to previous version: %w", err)
	}

	return nil
}

func getPlatformAssetName() string {
	os := runtime.GOOS
	arch := runtime.GOARCH

	switch os {
	case "linux":
		switch arch {
		case "amd64":
			return "motd-linux-amd64"
		case "arm64":
			return "motd-linux-arm64"
		}
	case "darwin":
		switch arch {
		case "amd64":
			return "motd-darwin-amd64"
		case "arm64":
			return "motd-darwin-arm64"
		}
	case "windows":
		if arch == "amd64" {
			return "motd-windows-amd64.exe"
		}
	}
	return ""
}

func getChecksums(release *GitHubRelease) (map[string]string, error) {
	var checksumsURL string
	for _, asset := range release.Assets {
		if asset.Name == "checksums.txt" {
			checksumsURL = asset.URL
			break
		}
	}

	if checksumsURL == "" {
		return nil, fmt.Errorf("checksums.txt not found in release assets")
	}

	resp, err := httpClient.Get(checksumsURL)
	if err != nil {
		return nil, fmt.Errorf("failed to download checksums: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("checksums download returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read checksums: %w", err)
	}

	checksums := make(map[string]string)
	lines := strings.Split(string(body), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			checksums[parts[1]] = parts[0]
		}
	}

	return checksums, nil
}

func downloadBinary(url, filename string, checksums map[string]string) (string, error) {
	// Create temp file
	tempFile, err := os.CreateTemp("", "motd-update-*.tmp")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	defer tempFile.Close()

	// Download file
	resp, err := httpClient.Get(url)
	if err != nil {
		return "", fmt.Errorf("failed to download binary: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download returned status %d", resp.StatusCode)
	}

	// Calculate checksum while downloading
	hasher := sha256.New()
	multiWriter := io.MultiWriter(tempFile, hasher)

	if _, err := io.Copy(multiWriter, resp.Body); err != nil {
		return "", fmt.Errorf("failed to save binary: %w", err)
	}

	// Verify checksum
	downloadedChecksum := fmt.Sprintf("%x", hasher.Sum(nil))
	expectedChecksum, exists := checksums[filename]
	if !exists {
		return "", fmt.Errorf("no checksum found for %s", filename)
	}

	if downloadedChecksum != expectedChecksum {
		return "", fmt.Errorf("checksum verification failed: expected %s, got %s", expectedChecksum, downloadedChecksum)
	}

	return tempFile.Name(), nil
}

func replaceBinary(tempPath, execPath string) error {
	// Make sure the temp file is executable
	if err := os.Chmod(tempPath, 0755); err != nil {
		return fmt.Errorf("failed to make binary executable: %w", err)
	}

	// On Unix systems, we can replace the running binary
	// On Windows, we need to use a different approach
	if runtime.GOOS == "windows" {
		// For Windows, we need to schedule the replacement for after the process exits
		// This is a simplified approach - in production, you might want to use a more robust method
		batPath := filepath.Join(os.TempDir(), "motd-update.bat")
		batContent := fmt.Sprintf(`@echo off
timeout /t 1 /nobreak >nul
move /Y "%s" "%s"
del "%s"
`, tempPath, execPath, batPath)

		if err := os.WriteFile(batPath, []byte(batContent), 0644); err != nil {
			return fmt.Errorf("failed to create update script: %w", err)
		}

		cmd := exec.Command("cmd", "/c", batPath)
		if err := cmd.Start(); err != nil {
			return fmt.Errorf("failed to start update script: %w", err)
		}

		fmt.Printf("%sUpdate scheduled. The binary will be replaced when this process exits.%s\n", YELLOW, RESET)
		return nil
	}

	// For Unix systems, replace the binary directly
	if err := os.Rename(tempPath, execPath); err != nil {
		return fmt.Errorf("failed to replace binary: %w", err)
	}

	return nil
}

// Test feature for version increment verification
// Minor fix for testing release workflow
// CI test commit - should not trigger release
