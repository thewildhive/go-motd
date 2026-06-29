package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"motd/config"
	"motd/display"
	"motd/media"
	"motd/system"
)

type wizardService struct {
	DisplayName       string
	DefaultURL        string
	DefaultInstance   string
	CredentialField   string
	CredentialDefault string
	Slice             *[]config.ServiceConfig
}

func handleConfigure() {
	cfgPath := config.GetConfigPaths()[0]
	reader := bufio.NewReader(os.Stdin)

	// Determine if config exists on disk
	configExists := false
	for _, p := range config.GetConfigPaths() {
		if _, err := os.Stat(p); err == nil {
			cfgPath = p
			configExists = true
			break
		}
	}

	// Check write access to the config directory before prompting the user.
	dir := filepath.Dir(cfgPath)
	if err := checkWriteAccess(dir); err != nil {
		fmt.Printf("%sCannot write to %s%s\n", display.Red, dir, display.Reset)
		fmt.Printf("%sCreate the file manually or use a writable path with -config.%s\n", display.Yellow, display.Reset)
		os.Exit(1)
	}

	// Load existing config or start fresh
	cfg, err := config.Load("", false, func(string, ...interface{}) {})
	if err != nil {
		if _, ok := err.(*config.LegacyConfigError); ok {
			fmt.Printf("%sLegacy YAML config detected. Run 'motd -migrate' first.%s\n", display.Red, display.Reset)
			os.Exit(1)
		}
		cfg = config.Config{}
	}

	if configExists {
		fmt.Printf("Editing existing config at %s%s%s\n", display.Cyan, cfgPath, display.Reset)
	} else {
		fmt.Printf("No existing config found. Creating new config at %s%s%s\n", display.Cyan, cfgPath, display.Reset)
	}
	fmt.Println()

	// --- System Setup ---
	fmt.Printf("%s━━━ System Setup ━━━%s\n", display.Bold, display.Reset)

	iface := system.GetDefaultInterface()
	if iface == "" {
		iface = "eth0"
	}
	cfg.System.Network.Interface = prompt(reader, "Network interface", cfg.System.Network.Interface, iface)
	cfg.System.TankMount = prompt(reader, "Tank mount path (leave empty to skip)", cfg.System.TankMount, "")

	// --- Service Setup ---
	fmt.Printf("\n%s━━━ Service Setup ── toggle services on/off ──%s\n", display.Bold, display.Reset)

	services := []wizardService{
		{"Plex", "http://localhost:32400", "Main", "token", "", &cfg.Services.Plex},
		{"Jellyfin", "http://localhost:8096", "Main", "token", "", &cfg.Services.Jellyfin},
		{"Sonarr", "http://localhost:8989", "HD", "api_key", "", &cfg.Services.Sonarr},
		{"Radarr", "http://localhost:7878", "HD", "api_key", "", &cfg.Services.Radarr},
		{"Seerr", "http://localhost:5055", "Main", "api_key", "", &cfg.Services.Seerr},
	}
	for _, ws := range services {
		configureServiceSlice(reader, &cfg, ws)
	}

	// --- Write ---
	if err := config.Write(cfgPath, cfg); err != nil {
		fmt.Printf("%sError saving configuration: %v%s\n", display.Red, err, display.Reset)
		os.Exit(1)
	}

	fmt.Printf("\n%sConfiguration saved to %s%s\n", display.Green, cfgPath, display.Reset)
}

// configureServiceSlice adds or modifies service instances defined by ws.
func configureServiceSlice(reader *bufio.Reader, cfg *config.Config, ws wizardService) {
	slice := ws.Slice
	if len(*slice) == 0 {
		if !promptBool(reader, fmt.Sprintf("Setup %s", ws.DisplayName), false) {
			return
		}
		*slice = append(*slice, config.ServiceConfig{Enabled: true})
		promptInstance(reader, &(*slice)[0], ws)
		return
	}

	fmt.Printf("\n%s configured:\n", ws.DisplayName)
	for i := range *slice {
		fmt.Printf("  Instance %d: %s %s\n", i+1, (*slice)[i].Name, (*slice)[i].URL)
	}

	action := promptUpdateAction(reader, ws.DisplayName)
	switch action {
	case "skip":
		return
	case "delete":
		for i := len(*slice) - 1; i >= 0; i-- {
			fmt.Printf("  Delete instance %d: %s %s? [y/N]: ", i+1, (*slice)[i].Name, (*slice)[i].URL)
			input := readLine(reader)
			if strings.ToLower(strings.TrimSpace(input)) == "y" {
				*slice = append((*slice)[:i], (*slice)[i+1:]...)
			}
		}
		return
	default: // "update"
		for i := range *slice {
			fmt.Printf("  --- Instance %d ---\n", i+1)
			promptInstance(reader, &(*slice)[i], ws)
		}
	}
}

func promptInstance(reader *bufio.Reader, svc *config.ServiceConfig, ws wizardService) {
	promptString(reader, svc, "name", ws.DefaultInstance)
	promptString(reader, svc, "url", ws.DefaultURL)
	promptCredential(reader, svc, ws.CredentialField, ws.CredentialDefault)
}

// readLine reads a single line from the reader, trimming whitespace.
func readLine(reader *bufio.Reader) string {
	input, err := reader.ReadString('\n')
	if err != nil {
		return ""
	}
	return strings.TrimSpace(input)
}

// prompt asks a question and returns the trimmed answer.
func prompt(reader *bufio.Reader, label, currentVal, fallback string) string {
	displayVal := currentVal
	if displayVal == "" {
		displayVal = fallback
	}

	fmt.Printf("%s [%s]: ", label, displayVal)
	input := readLine(reader)
	if input == "" {
		return displayVal
	}
	return input
}

// promptUpdateAction asks whether to update, skip, or delete a service.
// Returns "update", "skip", or "delete".
func promptUpdateAction(reader *bufio.Reader, name string) string {
	fmt.Printf("Update/delete %s [Y/n/d]: ", name)
	input := readLine(reader)
	input = strings.ToLower(input)
	switch input {
	case "d", "del", "delete":
		return "delete"
	case "n", "no":
		return "skip"
	default:
		return "update"
	}
}

// promptBool asks a yes/no question.
func promptBool(reader *bufio.Reader, label string, defaultVal bool) bool {
	suffix := "y/N"
	if defaultVal {
		suffix = "Y/n"
	}
	fmt.Printf("%s [%s]: ", label, suffix)
	input := readLine(reader)
	input = strings.ToLower(input)
	switch input {
	case "y", "yes":
		return true
	case "n", "no":
		return false
	default:
		return defaultVal
	}
}

// promptString prompts for a service field.
func promptString(reader *bufio.Reader, svc *config.ServiceConfig, field, fallback string) {
	current := fieldValue(svc, field)
	answer := prompt(reader, "  "+field, current, fallback)
	setFieldValue(svc, field, answer)
	if field == "url" && media.IsPlaintextToRemote(answer) {
		fmt.Printf("  %sWarning: API key/token will be sent in plaintext over HTTP to %s%s\n", display.Yellow, answer, display.Reset)
	}
}

// promptCredential is like promptString but masks the current value.
func promptCredential(reader *bufio.Reader, svc *config.ServiceConfig, field, fallback string) {
	current := fieldValue(svc, field)
	displayVal := current
	if displayVal != "" {
		displayVal = "******"
	}

	fmt.Printf("  %s [%s]: ", field, displayVal)
	input := readLine(reader)

	if input == "" {
		if current != "" {
			return
		}
		if fallback != "" {
			setFieldValue(svc, field, fallback)
		}
		return
	}
	setFieldValue(svc, field, input)
}

func fieldValue(svc *config.ServiceConfig, field string) string {
	switch field {
	case "name":
		return svc.Name
	case "url":
		return svc.URL
	case "token":
		return svc.Token
	case "api_key":
		return svc.APIKey
	default:
		return ""
	}
}

func setFieldValue(svc *config.ServiceConfig, field, value string) {
	switch field {
	case "name":
		svc.Name = value
	case "url":
		svc.URL = value
	case "token":
		svc.Token = value
	case "api_key":
		svc.APIKey = value
	}
}

func checkWriteAccess(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmpFile, err := os.CreateTemp(dir, ".motd-write-test-*")
	if err != nil {
		return err
	}
	tmpFile.Close()
	return os.Remove(tmpFile.Name())
}
