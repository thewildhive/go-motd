package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"motd/config"
	"motd/display"
)

var errNoLegacyConfig = errors.New("no legacy YAML config files found")

func runConfigMigration(configPath string) error {
	legacyPath, jsonPath, unsupportedServices, err := migrateLegacyConfig(configPath)
	if err != nil {
		return err
	}

	fmt.Println("Migrated legacy YAML config to JSON.")
	fmt.Printf("Source: %s\n", legacyPath)
	fmt.Printf("Target: %s\n", jsonPath)
	if len(unsupportedServices) > 0 {
		fmt.Printf("Skipped unsupported legacy services: %s\n", strings.Join(unsupportedServices, ", "))
	}

	return nil
}

func printConfigMigrationError(err error) {
	if errors.Is(err, errNoLegacyConfig) {
		fmt.Printf("%sError: No legacy YAML config found to migrate.%s\n", display.Red, display.Reset)
		fmt.Println("Expected config.yml or config.yaml next to the target JSON config path.")
		return
	}

	fmt.Printf("%sError migrating configuration: %v%s\n", display.Red, err, display.Reset)
}

func migrateLegacyConfig(configPath string) (string, string, []string, error) {
	jsonPaths := config.GetConfigPaths()
	legacyPaths := config.GetLegacyConfigPaths()
	if strings.TrimSpace(configPath) != "" {
		jsonPaths = []string{configPath}
		legacyPaths = config.GetExplicitLegacyConfigPaths(configPath)
	}

	return migrateLegacyConfigFromPaths(jsonPaths, legacyPaths)
}

func migrateLegacyConfigFromPaths(jsonPaths, legacyPaths []string) (string, string, []string, error) {
	var data []byte
	var legacyPath string

	for _, lp := range legacyPaths {
		if d, err := os.ReadFile(lp); err == nil {
			data = d
			legacyPath = lp
			break
		}
	}

	if data == nil {
		return "", "", nil, errNoLegacyConfig
	}

	parsedConfig, unsupportedServices, err := parseLegacyYAMLConfig(data)
	if err != nil {
		return "", "", nil, fmt.Errorf("failed to parse legacy config: %w", err)
	}

	jsonPath := legacyPath
	if strings.HasSuffix(legacyPath, ".yml") || strings.HasSuffix(legacyPath, ".yaml") {
		dir := filepath.Dir(legacyPath)
		name := filepath.Base(legacyPath)
		ext := filepath.Ext(name)
		name = strings.TrimSuffix(name, ext)

		jsonPath = filepath.Join(dir, name+".json")
	}

	if len(jsonPaths) > 0 &&
		filepath.Dir(jsonPaths[0]) == filepath.Dir(legacyPath) {
		jsonPath = jsonPaths[0]
	}

	if _, err := os.Stat(jsonPath); err == nil {
		return "", "", nil, fmt.Errorf("target JSON config already exists: %s", jsonPath)
	}

	jsonData, err := json.MarshalIndent(parsedConfig, "", "  ")
	if err != nil {
		return "", "", nil, fmt.Errorf("failed to marshal JSON: %w", err)
	}

	jsonData = append(jsonData, '\n')

	if _, err := config.DecodeJSONConfig(jsonData); err != nil {
		return legacyPath, jsonPath, unsupportedServices, fmt.Errorf("generated JSON config is invalid: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(jsonPath), 0o755); err != nil {
		return legacyPath, jsonPath, unsupportedServices, fmt.Errorf("failed to create JSON config directory %s: %w", filepath.Dir(jsonPath), err)
	}
	if err := os.WriteFile(jsonPath, jsonData, 0o600); err != nil {
		return legacyPath, jsonPath, unsupportedServices, fmt.Errorf("failed to write JSON config %s: %w", jsonPath, err)
	}

	return legacyPath, jsonPath, unsupportedServices, nil
}

func parseLegacyYAMLConfig(data []byte) (config.Config, []string, error) {
	parsedConfig := newMigratedConfig()
	section := ""
	currentService := ""
	currentServiceIndex := -1
	unsupportedServices := []string{}

	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")

	// Detect indentation step from the first non-zero-indented line.
	// Supports both 2-space and 4-space indentation (and any other
	// consistent multiple). Lines are normalized to 2-space levels.
	indentStep := 2
	for _, candidate := range lines {
		if strings.TrimSpace(candidate) == "" || strings.TrimSpace(candidate) == "---" {
			continue
		}
		if n := countLeadingSpaces(candidate); n > 0 {
			indentStep = n
			break
		}
	}

	// normalizeIndent converts raw indentation to 2-space-equivalent levels.
	// For 4-space indentation: raw 0→0, 4→2, 8→4, 12→6, etc.
	normalizeIndent := func(raw int) int {
		if indentStep <= 2 || raw == 0 {
			return raw
		}
		return (raw / indentStep) * 2
	}
	for lineNumber, rawLine := range lines {
		var err error

		if strings.Contains(rawLine, "\t") {
			return parsedConfig, unsupportedServices, fmt.Errorf("line %d: tabs are not supported", lineNumber+1)
		}

		line := stripLegacyYAMLComment(strings.TrimRight(rawLine, " \r"))
		if strings.TrimSpace(line) == "" {
			continue
		}

		indent := normalizeIndent(countLeadingSpaces(line))

		if indent == 0 {
			section = ""
			currentService = ""
			currentServiceIndex = -1

			trimmed := strings.TrimSpace(line)
			key, _, hasValue := splitLegacyYAMLKeyValue(trimmed)
			if key != "" && !hasValue {
				if key == "services" || key == "system" || strings.EqualFold(key, "Organizr") {
					section = key
					continue
				}
			}
		}

		if section == "services" || strings.EqualFold(section, "Organizr") {
			currentService, currentServiceIndex, err = processServiceLine(
				&parsedConfig, &unsupportedServices, section,
				line, indent, currentService, currentServiceIndex,
			)
			if err != nil {
				return parsedConfig, unsupportedServices, err
			}
			continue
		}

		if section == "system" {
			if indent == 2 {
				trimmed := strings.TrimSpace(line)
				key, value, hasValue := splitLegacyYAMLKeyValue(trimmed)
				if !hasValue {
					continue
				}
				if err := setSystemConfigField(&parsedConfig, key, value); err != nil {
					return parsedConfig, unsupportedServices, err
				}
			}
			if indent == 4 {
				trimmed := strings.TrimSpace(line)
				key, value, hasValue := splitLegacyYAMLKeyValue(trimmed)
				if !hasValue {
					continue
				}
				// Nested system keys like network.interface
				if key == "interface" {
					interfaceName, err := parseLegacyYAMLString(value)
					if err != nil {
						return parsedConfig, unsupportedServices, err
					}
					parsedConfig.System.Network.Interface = interfaceName
				}
			}
		}
	}

	return parsedConfig, unsupportedServices, nil
}

func processServiceLine(parsedConfig *config.Config, unsupportedServices *[]string, section, line string, indent int, currentService string, currentServiceIndex int) (string, int, error) {
	trimmed := strings.TrimSpace(line)

	switch indent {
	case 2:
		key, value, hasValue := splitLegacyYAMLKeyValue(trimmed)

		// Check for organizr (even with [] shorthand value)
		if strings.EqualFold(key, "Organizr") {
			*unsupportedServices = appendUniqueString(*unsupportedServices, "organizr")
			return "", -1, nil
		}

		if value == "[]" || value == "{}" {
			return key, -1, nil
		}

		if !hasValue {
			// Start of a service type block
			inService := isValidLegacyService(key)
			if !inService {
				return currentService, currentServiceIndex, nil
			}
			currentService = key
			currentServiceIndex = -1
		}
		return currentService, currentServiceIndex, nil

	case 4:
		if strings.HasPrefix(trimmed, "- ") {
			if currentService == "" {
				return currentService, currentServiceIndex, nil
			}
			currentServiceIndex = appendLegacyService(parsedConfig, currentService)

			// Extract inline field from "- key: value" if present
			itemContent := strings.TrimSpace(strings.TrimPrefix(trimmed, "-"))
			if itemContent != "" {
				setLegacyServiceField(parsedConfig, currentService, currentServiceIndex, itemContent, "")
			}

			return currentService, currentServiceIndex, nil
		}

		// Current service field — requires a list item.
		// If there's no current service or list index, silently skip.
		// This handles malformed YAML where fields appear outside a list.
		if currentService == "" || currentServiceIndex < 0 {
			return currentService, currentServiceIndex, nil
		}

		if err := setLegacyServiceField(parsedConfig, currentService, currentServiceIndex, trimmed, line); err != nil {
			return currentService, currentServiceIndex, err
		}
		return currentService, currentServiceIndex, nil

	default:
		if indent > 4 {
			if currentService == "" || currentServiceIndex < 0 {
				return currentService, currentServiceIndex, nil
			}
			if err := setLegacyServiceField(parsedConfig, currentService, currentServiceIndex, trimmed, line); err != nil {
				return currentService, currentServiceIndex, err
			}
		}
		return currentService, currentServiceIndex, nil
	}
}

func isValidLegacyService(key string) bool {
	switch strings.ToLower(key) {
	case "plex", "jellyfin", "sonarr", "radarr", "seerr":
		return true
	}
	return false
}

func setSystemConfigField(parsedConfig *config.Config, key, value string) error {
	switch key {
	case "compose_dir":
		parsedConfig.System.ComposeDir = strings.Trim(value, "\"'")
	case "tank_mount":
		parsedConfig.System.TankMount = strings.Trim(value, "\"'")
	default:
		parsedConfig.System.Network.Interface = strings.Trim(value, "\"'")
	}
	return nil
}

func newMigratedConfig() config.Config {
	return config.Config{}
}

func legacyServiceSlice(parsedConfig *config.Config, service string) *[]config.ServiceConfig {
	switch strings.ToLower(service) {
	case "plex":
		return &parsedConfig.Services.Plex
	case "jellyfin":
		return &parsedConfig.Services.Jellyfin
	case "sonarr":
		return &parsedConfig.Services.Sonarr
	case "radarr":
		return &parsedConfig.Services.Radarr
	case "seerr":
		return &parsedConfig.Services.Seerr
	default:
		return nil
	}
}

func appendLegacyService(parsedConfig *config.Config, service string) int {
	slice := legacyServiceSlice(parsedConfig, service)
	if slice == nil {
		return 0
	}
	*slice = append(*slice, config.ServiceConfig{})
	return len(*slice) - 1
}

func setLegacyServiceField(parsedConfig *config.Config, service string, index int, trimmed, line string) error {
	colonIdx := strings.Index(trimmed, ":")
	if colonIdx < 0 {
		return nil
	}
	key := strings.TrimSpace(trimmed[:colonIdx])
	rawValue := strings.TrimSpace(trimmed[colonIdx+1:])

	services := legacyServiceSlice(parsedConfig, service)
	if services == nil || index < 0 || index >= len(*services) {
		return fmt.Errorf("service field outside list item")
	}

	serviceConfig := &(*services)[index]
	switch key {
	case "name":
		parsedValue, err := parseLegacyYAMLString(rawValue)
		if err != nil {
			return err
		}
		serviceConfig.Name = parsedValue
	case "url":
		parsedValue, err := parseLegacyYAMLString(rawValue)
		if err != nil {
			return err
		}
		serviceConfig.URL = parsedValue
	case "api_key":
		parsedValue, err := parseLegacyYAMLString(rawValue)
		if err != nil {
			return err
		}
		serviceConfig.APIKey = parsedValue
	case "apikey":
		parsedValue, err := parseLegacyYAMLString(rawValue)
		if err != nil {
			return err
		}
		serviceConfig.APIKey = parsedValue
	case "token":
		parsedValue, err := parseLegacyYAMLString(rawValue)
		if err != nil {
			return err
		}
		serviceConfig.Token = parsedValue
	case "enabled":
		parsedValue, err := parseLegacyYAMLBool(rawValue)
		if err != nil {
			return err
		}
		serviceConfig.Enabled = parsedValue
	default:
		return fmt.Errorf("unsupported service field %q", key)
	}

	return nil
}

func stripLegacyYAMLComment(line string) string {
	if idx := strings.Index(line, "#"); idx >= 0 {
		line = line[:idx]
	}
	return line
}

func splitLegacyYAMLKeyValue(content string) (string, string, bool) {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return "", "", false
	}
	colonIdx := strings.Index(trimmed, ":")
	if colonIdx < 0 {
		return trimmed, "", false
	}

	key := strings.TrimSpace(trimmed[:colonIdx])
	value := strings.TrimSpace(trimmed[colonIdx+1:])

	return key, value, len(value) > 0
}

func parseLegacyYAMLString(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("empty string")
	}
	// Strip surrounding quotes
	value = strings.Trim(value, "\"'")
	return value, nil
}

func parseLegacyYAMLBool(value string) (bool, error) {
	value = strings.TrimSpace(strings.ToLower(value))
	switch value {
	case "true", "yes", "1", "on":
		return true, nil
	case "false", "no", "0", "off":
		return false, nil
	default:
		return false, fmt.Errorf("invalid boolean value: %s", value)
	}
}

func countLeadingSpaces(value string) int {
	count := 0
	for _, ch := range value {
		if ch == ' ' {
			count++
		} else {
			break
		}
	}
	return count
}

func appendUniqueString(values []string, value string) []string {
	for _, v := range values {
		if v == value {
			return values
		}
	}
	return append(values, value)
}
