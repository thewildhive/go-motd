package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"encoding/xml"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

var VERSION = "0.2.5"

const (
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
	Name    string `json:"name"`
	URL     string `json:"url"`
	APIKey  string `json:"api_key,omitempty"`
	Token   string `json:"token,omitempty"`
	Enabled bool   `json:"enabled"`
}

// Config holds application configuration
type Config struct {
	Services struct {
		Plex     []ServiceConfig `json:"plex"`
		Jellyfin []ServiceConfig `json:"jellyfin"`
		Sonarr   []ServiceConfig `json:"sonarr"`
		Radarr   []ServiceConfig `json:"radarr"`
		Seerr    []ServiceConfig `json:"seerr"`
	} `json:"services"`
	System struct {
		ComposeDir string `json:"compose_dir"`
		TankMount  string `json:"tank_mount"`
		Network    struct {
			Interface string `json:"interface,omitempty"`
		} `json:"network,omitempty"`
	} `json:"system"`
}

type vnstatInterface struct {
	ID      string `json:"id"`
	Traffic struct {
		Month []vnstatMonthlyEntry `json:"month"`
	} `json:"traffic"`
}

// Global state
var (
	config     Config
	httpClient *http.Client
	debugMode  bool
)

var errNoJSONConfig = errors.New("no JSON config files found")

type legacyConfigError struct {
	legacyPath   string
	requiredPath string
	fallbackPath string
}

func (e *legacyConfigError) Error() string {
	return fmt.Sprintf("legacy YAML config detected at %s", e.legacyPath)
}

func getConfigPaths() []string {
	home := getUserHome()
	userConfig := filepath.Join(home, ".config", "motd", "config.json")
	if home == "" {
		return []string{"/opt/motd/config.json"}
	}
	return []string{userConfig, "/opt/motd/config.json"}
}

func getLegacyConfigPaths() []string {
	home := getUserHome()
	userYML := filepath.Join(home, ".config", "motd", "config.yml")
	userYAML := filepath.Join(home, ".config", "motd", "config.yaml")
	if home == "" {
		return []string{"/opt/motd/config.yml", "/opt/motd/config.yaml"}
	}
	return []string{userYML, userYAML, "/opt/motd/config.yml", "/opt/motd/config.yaml"}
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
		showSeerr()
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
  ~/.config/motd/config.json   (highest priority)
  /opt/motd/config.json        (fallback)`)
}

func debugLog(msg string, args ...interface{}) {
	if debugMode {
		fmt.Fprintf(os.Stderr, "[DEBUG] "+msg+"\n", args...)
	}
}

func decodeJSONConfig(data []byte) (Config, error) {
	var parsedConfig Config

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&parsedConfig); err != nil {
		return parsedConfig, err
	}

	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return parsedConfig, fmt.Errorf("config file must contain a single JSON object")
		}
		return parsedConfig, err
	}

	return parsedConfig, nil
}

func loadJSONConfigFromPaths(paths []string) (Config, error) {
	var loadedConfig Config

	for _, configPath := range paths {
		info, err := os.Stat(configPath)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return loadedConfig, fmt.Errorf("failed to stat config file %s: %w", configPath, err)
		}

		if info.IsDir() {
			return loadedConfig, fmt.Errorf("config file path is a directory: %s", configPath)
		}

		debugLog("Loading JSON config from: %s", configPath)
		data, err := os.ReadFile(configPath)
		if err != nil {
			return loadedConfig, fmt.Errorf("failed to read config file %s: %w", configPath, err)
		}

		parsedConfig, err := decodeJSONConfig(data)
		if err != nil {
			return loadedConfig, fmt.Errorf("failed to parse JSON config %s: %w", configPath, err)
		}

		debugLog("Successfully loaded JSON config from: %s", configPath)
		return parsedConfig, nil
	}

	debugLog("No JSON config files found")
	return loadedConfig, errNoJSONConfig
}

func detectLegacyYAMLConfigFromPaths(legacyPaths, jsonPaths []string) *legacyConfigError {
	fallbackPath := ""
	if len(jsonPaths) > 1 {
		fallbackPath = jsonPaths[1]
	}

	for i, legacyPath := range legacyPaths {
		if _, err := os.Stat(legacyPath); err == nil {
			requiredPath := ""
			if i < len(jsonPaths) {
				requiredPath = jsonPaths[i]
			} else if len(jsonPaths) > 0 {
				requiredPath = jsonPaths[0]
			}

			return &legacyConfigError{
				legacyPath:   legacyPath,
				requiredPath: requiredPath,
				fallbackPath: fallbackPath,
			}
		}
	}

	return nil
}

func loadRuntimeConfigFromPaths(jsonPaths, legacyPaths []string) (Config, error) {
	loadedConfig, err := loadJSONConfigFromPaths(jsonPaths)
	if err == nil {
		return loadedConfig, nil
	}

	if errors.Is(err, errNoJSONConfig) {
		if legacyErr := detectLegacyYAMLConfigFromPaths(legacyPaths, jsonPaths); legacyErr != nil {
			return Config{}, legacyErr
		}
	}

	return Config{}, err
}

func loadRuntimeConfig() (Config, error) {
	return loadRuntimeConfigFromPaths(getConfigPaths(), getLegacyConfigPaths())
}

func printMissingConfigError(paths []string) {
	fmt.Printf("%sError: No configuration file found. Please create a JSON config file at:%s\n", RED, RESET)
	for _, path := range paths {
		fmt.Printf("  %s\n", path)
	}
}

func printLegacyConfigError(err *legacyConfigError) {
	fmt.Printf("%sError: Legacy YAML config is no longer supported.%s\n", RED, RESET)
	fmt.Printf("Found legacy config at: %s\n", err.legacyPath)
	if err.requiredPath != "" {
		fmt.Printf("Create a JSON config at: %s\n", err.requiredPath)
	}
	if err.fallbackPath != "" {
		fmt.Printf("Fallback JSON path: %s\n", err.fallbackPath)
	}
}

func loadConfig() {
	loadedConfig, err := loadRuntimeConfig()
	if err != nil {
		if errors.Is(err, errNoJSONConfig) {
			printMissingConfigError(getConfigPaths())
			os.Exit(1)
		}

		var legacyErr *legacyConfigError
		if errors.As(err, &legacyErr) {
			printLegacyConfigError(legacyErr)
			os.Exit(1)
		}

		fmt.Printf("%sError loading configuration: %v%s\n", RED, err, RESET)
		os.Exit(1)
	}

	config = loadedConfig
	debugLog("Using JSON configuration")
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

	for _, seerr := range config.Services.Seerr {
		if seerr.Enabled && (seerr.APIKey != "") {
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

type vnstatMonthlyEntry struct {
	Rx   uint64 `json:"rx"`
	Tx   uint64 `json:"tx"`
	Date struct {
		Year  int `json:"year"`
		Month int `json:"month"`
	} `json:"date"`
}

type vnstatData struct {
	Interfaces []vnstatInterface `json:"interfaces"`
}

func pickLatestVnstatMonth(entries []vnstatMonthlyEntry, now time.Time) (vnstatMonthlyEntry, bool) {
	if len(entries) == 0 {
		return vnstatMonthlyEntry{}, false
	}

	for _, entry := range entries {
		if entry.Date.Year == now.Year() && entry.Date.Month == int(now.Month()) {
			return entry, true
		}
	}

	latest := entries[0]
	for _, entry := range entries[1:] {
		if entry.Date.Year > latest.Date.Year || (entry.Date.Year == latest.Date.Year && entry.Date.Month > latest.Date.Month) {
			latest = entry
		}
	}

	return latest, true
}

func pickVnstatInterface(data vnstatData, preferred string) (vnstatInterface, bool) {
	if len(data.Interfaces) == 0 {
		return vnstatInterface{}, false
	}

	if preferred != "" {
		for _, iface := range data.Interfaces {
			if iface.ID == preferred {
				return iface, true
			}
		}
	}

	for _, iface := range data.Interfaces {
		if len(iface.Traffic.Month) > 0 {
			return iface, true
		}
	}

	return data.Interfaces[0], true
}

func parseVnstatMonthlyUsage(output []byte, preferredInterface string, now time.Time) (float64, float64, float64, float64, error) {
	var parsed vnstatData
	if err := json.Unmarshal(output, &parsed); err != nil {
		return 0, 0, 0, 0, err
	}

	iface, ok := pickVnstatInterface(parsed, preferredInterface)
	if !ok || len(iface.Traffic.Month) == 0 {
		return 0, 0, 0, 0, fmt.Errorf("no vnstat interface/monthly data available")
	}

	month, ok := pickLatestVnstatMonth(iface.Traffic.Month, now)
	if !ok {
		return 0, 0, 0, 0, fmt.Errorf("no vnstat monthly entry available")
	}

	rxGB := float64(month.Rx) / 1073741824.0
	txGB := float64(month.Tx) / 1073741824.0

	day := float64(now.Day())
	if day < 1 {
		day = 1
	}

	rxEst := rxGB * (30.0 / day)
	txEst := txGB * (30.0 / day)

	return rxGB, txGB, rxEst, txEst, nil
}

func showBandwidth() {
	if !hasCommand("vnstat") {
		return
	}

	interfaceName := strings.TrimSpace(config.System.Network.Interface)
	if interfaceName == "" {
		interfaceName = getDefaultInterface()
	}
	if interfaceName == "" {
		interfaceName = "enp7s0"
	}

	output, err := exec.Command("vnstat", "--json", "m", "-i", interfaceName).Output()
	if err != nil {
		if strings.TrimSpace(config.System.Network.Interface) == "" {
			output, err = exec.Command("vnstat", "--json", "m").Output()
		}
	}

	if err != nil {
		debugLog("vnstat command failed: %v", err)
		return
	}

	rxGB, txGB, rxEst, txEst, err := parseVnstatMonthlyUsage(output, interfaceName, time.Now())
	if err != nil {
		debugLog("Failed to parse vnstat data for %s: %v", interfaceName, err)
		return
	}

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

type plexSessionsResponse struct {
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

type jellyfinSession struct {
	NowPlayingItem  json.RawMessage `json:"NowPlayingItem"`
	TranscodingInfo *struct {
		Bitrate int64 `json:"Bitrate"`
	} `json:"TranscodingInfo,omitempty"`
	PlayState struct {
		PlayMethod string `json:"PlayMethod"`
	} `json:"PlayState"`
}

type arrWantedMissingResponse struct {
	TotalRecords int               `json:"totalRecords"`
	Records      []json.RawMessage `json:"records"`
}

type seerrRequestCountResponse struct {
	Pending int `json:"pending"`
}

func fetchSeerrPendingCount(client *http.Client, baseURL, apiKey string) (int, error) {
	req, err := http.NewRequest("GET", serviceURL(baseURL, "/api/v1/request/count"), nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("X-Api-Key", apiKey)

	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	var result seerrRequestCountResponse
	if err := decodeJSONResponse(resp, &result); err != nil {
		return 0, err
	}

	return result.Pending, nil
}

func serviceLabel(base, name string) string {
	if name != "" && name != "Default" {
		return fmt.Sprintf("%s (%s)", base, name)
	}
	return base
}

func serviceURL(baseURL, path string) string {
	return strings.TrimRight(baseURL, "/") + path
}

func hasNowPlayingItem(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return false
	}
	return strings.TrimSpace(string(raw)) != "null"
}

func decodeJSONResponse(resp *http.Response, target interface{}) error {
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	decoder := json.NewDecoder(resp.Body)
	if err := decoder.Decode(target); err != nil {
		return err
	}

	return nil
}

func parseARRMissingCount(data arrWantedMissingResponse) int {
	if data.TotalRecords > 0 {
		return data.TotalRecords
	}
	return len(data.Records)
}

func parseJellyfinSessions(sessions []jellyfinSession) (int, int, float64, bool) {
	active := 0
	transcodes := 0
	totalBitrate := int64(0)
	hasBitrate := false

	for _, session := range sessions {
		if !hasNowPlayingItem(session.NowPlayingItem) {
			continue
		}

		active++
		if strings.EqualFold(session.PlayState.PlayMethod, "Transcode") {
			transcodes++
		}

		if session.TranscodingInfo != nil && session.TranscodingInfo.Bitrate > 0 {
			totalBitrate += session.TranscodingInfo.Bitrate
			hasBitrate = true
		}
	}

	mbps := float64(totalBitrate) / 1000000.0
	return active, transcodes, mbps, hasBitrate
}

func pluralSuffix(count int) string {
	if count == 1 {
		return ""
	}
	return "s"
}

func showPlex() {
	if len(config.Services.Plex) == 0 {
		return
	}

	for _, plex := range config.Services.Plex {
		if !plex.Enabled || plex.Token == "" {
			continue
		}
		showPlexInstance(plex)
	}
}

func showPlexInstance(plex ServiceConfig) {
	req, err := http.NewRequest("GET", serviceURL(plex.URL, "/status/sessions"), nil)
	if err != nil {
		debugLog("Plex request failed for %s: %v", plex.Name, err)
		return
	}
	req.Header.Set("X-Plex-Token", plex.Token)

	resp, err := httpClient.Do(req)
	if err != nil {
		debugLog("Plex request failed for %s: %v", plex.Name, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		debugLog("Plex returned status %d for %s", resp.StatusCode, plex.Name)
		return
	}

	var sessions plexSessionsResponse
	if err := xml.NewDecoder(resp.Body).Decode(&sessions); err != nil {
		debugLog("Failed to parse Plex XML for %s: %v", plex.Name, err)
		return
	}

	transcodes := 0
	bandwidth := 0
	for _, video := range sessions.Videos {
		if video.TranscodeSession.VideoDecision == "transcode" {
			transcodes++
		}
		bandwidth += video.Session.Bandwidth
	}

	label := serviceLabel("Plex", plex.Name)
	dotLabel(label)
	if sessions.Size == 0 {
		fmt.Printf("%sNo active streams%s\n", GREEN, RESET)
		return
	}

	bwMbps := float64(bandwidth) / 1000.0
	if transcodes == 0 {
		fmt.Printf("%s%d streams (%.2f Mbps)%s\n", YELLOW, sessions.Size, bwMbps, RESET)
		return
	}

	fmt.Printf("%s%d streams, %d transcodes (%.2f Mbps)%s\n", RED, sessions.Size, transcodes, bwMbps, RESET)
}

func showJellyfin() {
	if len(config.Services.Jellyfin) == 0 {
		return
	}

	for _, jellyfin := range config.Services.Jellyfin {
		if !jellyfin.Enabled || jellyfin.Token == "" {
			continue
		}
		showJellyfinInstance(jellyfin)
	}
}

func showJellyfinInstance(jellyfin ServiceConfig) {
	req, err := http.NewRequest("GET", serviceURL(jellyfin.URL, "/Sessions"), nil)
	if err != nil {
		debugLog("Jellyfin request build failed for %s: %v", jellyfin.Name, err)
		return
	}
	req.Header.Set("X-Emby-Token", jellyfin.Token)
	req.Header.Set("Authorization", "MediaBrowser Token=\""+jellyfin.Token+"\"")

	resp, err := httpClient.Do(req)
	if err != nil {
		debugLog("Jellyfin request failed for %s: %v", jellyfin.Name, err)
		return
	}
	defer resp.Body.Close()

	var sessions []jellyfinSession
	if err := decodeJSONResponse(resp, &sessions); err != nil {
		debugLog("Failed to decode Jellyfin response for %s: %v", jellyfin.Name, err)
		return
	}

	count, transcodes, bwMbps, hasBW := parseJellyfinSessions(sessions)
	label := serviceLabel("Jellyfin", jellyfin.Name)
	dotLabel(label)

	if count == 0 {
		fmt.Printf("%sNo active streams%s\n", GREEN, RESET)
		return
	}

	if transcodes == 0 {
		if hasBW {
			fmt.Printf("%s%d streams (%.2f Mbps)%s\n", YELLOW, count, bwMbps, RESET)
		} else {
			fmt.Printf("%s%d streams%s\n", YELLOW, count, RESET)
		}
		return
	}

	if hasBW {
		fmt.Printf("%s%d streams, %d transcodes (%.2f Mbps)%s\n", RED, count, transcodes, bwMbps, RESET)
		return
	}

	fmt.Printf("%s%d streams, %d transcodes%s\n", RED, count, transcodes, RESET)
}

func showSonarr() {
	if len(config.Services.Sonarr) == 0 {
		return
	}

	for _, sonarr := range config.Services.Sonarr {
		if !sonarr.Enabled || sonarr.APIKey == "" {
			continue
		}
		showSonarrInstance(sonarr)
	}
}

func showSonarrInstance(sonarr ServiceConfig) {
	req, err := http.NewRequest("GET", serviceURL(sonarr.URL, "/api/v3/wanted/missing"), nil)
	if err != nil {
		debugLog("Sonarr request build failed for %s: %v", sonarr.Name, err)
		return
	}
	req.Header.Set("X-Api-Key", sonarr.APIKey)

	resp, err := httpClient.Do(req)
	if err != nil {
		debugLog("Sonarr request failed for %s: %v", sonarr.Name, err)
		return
	}
	defer resp.Body.Close()

	var result arrWantedMissingResponse
	if err := decodeJSONResponse(resp, &result); err != nil {
		debugLog("Failed to decode Sonarr response for %s: %v", sonarr.Name, err)
		return
	}

	count := parseARRMissingCount(result)
	label := serviceLabel("Sonarr", sonarr.Name)
	dotLabel(label)
	if count == 0 {
		fmt.Printf("%sNo missing episodes%s\n", GREEN, RESET)
		return
	}

	fmt.Printf("%s%d missing episode%s%s\n", YELLOW, count, pluralSuffix(count), RESET)
}

func showRadarr() {
	if len(config.Services.Radarr) == 0 {
		return
	}

	for _, radarr := range config.Services.Radarr {
		if !radarr.Enabled || radarr.APIKey == "" {
			continue
		}
		showRadarrInstance(radarr)
	}
}

func showRadarrInstance(radarr ServiceConfig) {
	req, err := http.NewRequest("GET", serviceURL(radarr.URL, "/api/v3/wanted/missing"), nil)
	if err != nil {
		debugLog("Radarr request build failed for %s: %v", radarr.Name, err)
		return
	}
	req.Header.Set("X-Api-Key", radarr.APIKey)

	resp, err := httpClient.Do(req)
	if err != nil {
		debugLog("Radarr request failed for %s: %v", radarr.Name, err)
		return
	}
	defer resp.Body.Close()

	var result arrWantedMissingResponse
	if err := decodeJSONResponse(resp, &result); err != nil {
		debugLog("Failed to decode Radarr response for %s: %v", radarr.Name, err)
		return
	}

	count := parseARRMissingCount(result)
	label := serviceLabel("Radarr", radarr.Name)
	dotLabel(label)
	if count == 0 {
		fmt.Printf("%sNo missing movies%s\n", GREEN, RESET)
		return
	}

	fmt.Printf("%s%d missing movie%s%s\n", YELLOW, count, pluralSuffix(count), RESET)
}

func showSeerr() {
	if len(config.Services.Seerr) == 0 {
		return
	}

	for _, seerr := range config.Services.Seerr {
		if !seerr.Enabled || seerr.APIKey == "" {
			continue
		}
		showSeerrInstance(seerr)
	}
}

func showSeerrInstance(seerr ServiceConfig) {
	pending, err := fetchSeerrPendingCount(httpClient, seerr.URL, seerr.APIKey)
	if err != nil {
		debugLog("Seerr request failed for %s: %v", seerr.Name, err)
		return
	}

	label := serviceLabel("Seerr", seerr.Name)
	dotLabel(label)
	if pending == 0 {
		fmt.Printf("%sNo pending requests%s\n", GREEN, RESET)
		return
	}

	fmt.Printf("%s%d pending request%s%s\n", YELLOW, pending, pluralSuffix(pending), RESET)
}

// Helper functions
func getUserHome() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return home
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
