package main

import (
	"bufio"
	"encoding/json"
	"encoding/xml"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

const (
	VERSION        = "2.0.0"
	CURL_TIMEOUT   = 5 * time.Second
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

// Config holds environment configuration
type Config struct {
	PlexURL        string
	PlexToken      string
	JellyfinURL    string
	JellyfinToken  string
	SonarrURL      string
	SonarrAPIKey   string
	RadarrURL      string
	RadarrAPIKey   string
	OrganizrURL    string
	OrganizrAPIKey string
	ComposeDir     string
	TankMount      string
}

// Global state
var (
	config      Config
	httpClient  *http.Client
	debugMode   bool
	verboseMode bool
)

func main() {
	showHelp := flag.Bool("h", false, "Show help message")
	showVersion := flag.Bool("v", false, "Show version information")
	verbose := flag.Bool("V", false, "Show optional dependency warnings")
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

	verboseMode = *verbose
	debugMode = *debug

	// Initialize HTTP client
	httpClient = &http.Client{
		Timeout: CURL_TIMEOUT,
		Transport: &http.Transport{
			MaxIdleConns:        10,
			IdleConnTimeout:     30 * time.Second,
			DisableCompression:  false,
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
  -h    Show this help message
  -v    Show version information
  -V    Show optional dependency warnings
  -d    Enable debug mode

Environment Variables:
  ENV_FILE, PLEX_URL, PLEX_TOKEN, JELLYFIN_URL, JELLYFIN_TOKEN,
  SONARR_URL, SONARR_API_KEY, RADARR_URL, RADARR_API_KEY,
  ORGANIZR_URL, ORGANIZR_API_KEY, TANK_MOUNT, COMPOSEDIR`)
}

func debugLog(msg string, args ...interface{}) {
	if debugMode {
		fmt.Fprintf(os.Stderr, "[DEBUG] "+msg+"\n", args...)
	}
}

func loadConfig() {
	envFile := getEnv("ENV_FILE", "/opt/apps/compose/.env")
	if _, err := os.Stat(envFile); err == nil {
		loadEnvFile(envFile)
	}

	config = Config{
		PlexURL:        getEnv("PLEX_URL", "http://localhost:32400"),
		PlexToken:      getEnv("PLEX_TOKEN", ""),
		JellyfinURL:    getEnv("JELLYFIN_URL", "http://localhost:8096"),
		JellyfinToken:  getEnv("JELLYFIN_TOKEN", ""),
		SonarrURL:      getEnv("SONARR_URL", "http://localhost:8989"),
		SonarrAPIKey:   getEnv("SONARR_API_KEY", ""),
		RadarrURL:      getEnv("RADARR_URL", "http://localhost:7878"),
		RadarrAPIKey:   getEnv("RADARR_API_KEY", ""),
		OrganizrURL:    getEnv("ORGANIZR_URL", "http://localhost:XXXX"),
		OrganizrAPIKey: getEnv("ORGANIZR_API_KEY", ""),
		ComposeDir:     getEnv("COMPOSEDIR", "/opt/apps/compose"),
		TankMount:      getEnv("TANK_MOUNT", "/mnt/tank"),
	}
}

func loadEnvFile(path string) {
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.Trim(strings.TrimSpace(parts[1]), "\"'")
			os.Setenv(key, value)
		}
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func hasMediaServices() bool {
	return config.PlexToken != "" || config.JellyfinToken != "" ||
		config.SonarrAPIKey != "" || config.RadarrAPIKey != "" ||
		config.OrganizrAPIKey != ""
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
		fmt.Printf("%s%s╔══════════════════════════════════════╗%s\n", BOLD, CYAN, RESET)
		fmt.Printf("%s%s║  Connected to: %-20s ║%s\n", BOLD, CYAN, hostname, RESET)
		fmt.Printf("%s%s╚══════════════════════════════════════╝%s\n", BOLD, CYAN, RESET)
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

	output, err := exec.Command("vnstat", "--oneline", "b").Output()
	if err != nil || strings.Contains(string(output), "no data available") {
		return
	}

	parts := strings.Split(string(output), ";")
	if len(parts) >= 5 {
		rx, tx := parts[3], parts[4]
		
		rxVal, _ := strconv.ParseFloat(rx, 64)
		txVal, _ := strconv.ParseFloat(tx, 64)
		
		rxGB := rxVal / 1073741824.0
		txGB := txVal / 1073741824.0
		
		rxEst := (rxVal / 1073741824.0) * (30.0 / float64(time.Now().Day()))
		txEst := (txVal / 1073741824.0) * (30.0 / float64(time.Now().Day()))
		
		dotLabel("Bandwidth (rx)")
		fmt.Printf("%s%.2f GB / %.2f GB est%s\n", BLUE, rxGB, rxEst, RESET)
		dotLabel("Bandwidth (tx)")
		fmt.Printf("%s%.2f GB / %.2f GB est%s\n", BLUE, txGB, txEst, RESET)
	}
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
	if config.TankMount != "" {
		output, err := exec.Command("df", config.TankMount).Output()
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
					
					dotLabel(fmt.Sprintf("Disk (%s)", config.TankMount))
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
	if config.PlexToken == "" {
		return
	}

	url := fmt.Sprintf("%s/status/sessions?X-Plex-Token=%s", config.PlexURL, config.PlexToken)
	resp, err := httpClient.Get(url)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return
	}

	// Parse XML
	type Session struct {
		Size            int `xml:"size,attr"`
		VideoDecision   string
		SessionBandwidth int `xml:"Session>bandwidth,attr"`
	}

	type MediaContainer struct {
		Size  int `xml:"size,attr"`
		Videos []struct {
			TranscodeSession struct {
				VideoDecision string `xml:"videoDecision,attr"`
			} `xml:"TranscodeSession"`
			Session struct {
				Bandwidth int `xml:"bandwidth,attr"`
			} `xml:"Session"`
		} `xml:"Video"`
	}

	var container MediaContainer
	if err := xml.Unmarshal(body, &container); err == nil {
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

		dotLabel("Plex")
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
	if config.JellyfinToken == "" {
		return
	}

	req, err := http.NewRequest("GET", config.JellyfinURL+"/Sessions", nil)
	if err != nil {
		return
	}
	req.Header.Set("X-Emby-Token", config.JellyfinToken)

	resp, err := httpClient.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return
	}

	var sessions []map[string]interface{}
	if err := json.Unmarshal(body, &sessions); err != nil {
		return
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

	dotLabel("Jellyfin")
	if count == 0 {
		fmt.Printf("%sNo active streams%s\n", GREEN, RESET)
	} else if tcount == 0 {
		fmt.Printf("%s%d streams (%.2f Mbps)%s\n", YELLOW, count, bw, RESET)
	} else {
		fmt.Printf("%s%d streams, %d transcodes (%.2f Mbps)%s\n", RED, count, tcount, bw, RESET)
	}
}

func showSonarr() {
	if config.SonarrAPIKey == "" {
		return
	}

	url := fmt.Sprintf("%s/api/v3/wanted/missing?apikey=%s", config.SonarrURL, config.SonarrAPIKey)
	resp, err := httpClient.Get(url)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return
	}

	var result struct {
		Records []interface{} `json:"records"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return
	}

	count := len(result.Records)
	dotLabel("Sonarr")
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

func showRadarr() {
	if config.RadarrAPIKey == "" {
		return
	}

	url := fmt.Sprintf("%s/api/v3/wanted/missing?apikey=%s", config.RadarrURL, config.RadarrAPIKey)
	resp, err := httpClient.Get(url)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return
	}

	var result struct {
		Records []map[string]interface{} `json:"records"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return
	}

	count := 0
	for _, record := range result.Records {
		if isAvail, ok := record["isAvailable"].(bool); ok && isAvail {
			count++
		}
	}

	dotLabel("Radarr")
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

func showOrganizr() {
	if config.OrganizrAPIKey == "" {
		return
	}

	req, err := http.NewRequest("GET", config.OrganizrURL+"/api/v2/requests", nil)
	if err != nil {
		return
	}
	req.Header.Set("Authorization", "Bearer "+config.OrganizrAPIKey)

	resp, err := httpClient.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return
	}

	var result struct {
		Data struct {
			Total int `json:"total"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return
	}

	count := result.Data.Total
	dotLabel("Organizr")
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

// Helper functions
func hasCommand(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}

func hasFiglet() bool {
	return hasCommand("figlet")
}

func hasLolcat() bool {
	return hasCommand("lolcat")
}

func apiCurl(url string, headers map[string]string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}
