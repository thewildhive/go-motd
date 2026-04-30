package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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
		fmt.Printf("%sError: No legacy YAML config found to migrate.%s\n", RED, RESET)
		fmt.Println("Expected config.yml or config.yaml next to the target JSON config path.")
		return
	}

	fmt.Printf("%sError migrating configuration: %v%s\n", RED, err, RESET)
}

func migrateLegacyConfig(configPath string) (string, string, []string, error) {
	jsonPaths := getConfigPaths()
	legacyPaths := getLegacyConfigPaths()
	if strings.TrimSpace(configPath) != "" {
		jsonPaths = []string{configPath}
		legacyPaths = getExplicitLegacyConfigPaths(configPath)
	}

	return migrateLegacyConfigFromPaths(jsonPaths, legacyPaths)
}

func migrateLegacyConfigFromPaths(jsonPaths, legacyPaths []string) (string, string, []string, error) {
	legacyErr := detectLegacyYAMLConfigFromPaths(legacyPaths, jsonPaths)
	if legacyErr == nil {
		return "", "", nil, errNoLegacyConfig
	}
	if legacyErr.requiredPath == "" {
		return "", "", nil, fmt.Errorf("could not determine JSON target path for %s", legacyErr.legacyPath)
	}

	legacyPath := legacyErr.legacyPath
	jsonPath := legacyErr.requiredPath
	if filepath.Clean(legacyPath) == filepath.Clean(jsonPath) {
		return legacyPath, jsonPath, nil, fmt.Errorf("migration target must be a JSON path, not the legacy YAML path: %s", jsonPath)
	}

	info, err := os.Stat(legacyPath)
	if err != nil {
		return legacyPath, jsonPath, nil, fmt.Errorf("failed to stat legacy config %s: %w", legacyPath, err)
	}
	if info.IsDir() {
		return legacyPath, jsonPath, nil, fmt.Errorf("legacy config path is a directory: %s", legacyPath)
	}

	if info, err := os.Stat(jsonPath); err == nil {
		if info.IsDir() {
			return legacyPath, jsonPath, nil, fmt.Errorf("JSON config target is a directory: %s", jsonPath)
		}
		return legacyPath, jsonPath, nil, fmt.Errorf("JSON config already exists at %s", jsonPath)
	} else if !errors.Is(err, os.ErrNotExist) {
		return legacyPath, jsonPath, nil, fmt.Errorf("failed to stat JSON config target %s: %w", jsonPath, err)
	}

	data, err := os.ReadFile(legacyPath)
	if err != nil {
		return legacyPath, jsonPath, nil, fmt.Errorf("failed to read legacy config %s: %w", legacyPath, err)
	}

	parsedConfig, unsupportedServices, err := parseLegacyYAMLConfig(data)
	if err != nil {
		return legacyPath, jsonPath, unsupportedServices, fmt.Errorf("failed to parse legacy YAML config %s: %w", legacyPath, err)
	}

	jsonData, err := json.MarshalIndent(parsedConfig, "", "  ")
	if err != nil {
		return legacyPath, jsonPath, unsupportedServices, fmt.Errorf("failed to encode migrated JSON config: %w", err)
	}
	jsonData = append(jsonData, '\n')

	if _, err := decodeJSONConfig(jsonData); err != nil {
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

func parseLegacyYAMLConfig(data []byte) (Config, []string, error) {
	parsedConfig := newMigratedConfig()
	section := ""
	currentService := ""
	currentServiceIndex := -1
	unsupportedServices := []string{}

	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	for lineNumber, rawLine := range lines {
		var err error

		if strings.Contains(rawLine, "\t") {
			return parsedConfig, unsupportedServices, fmt.Errorf("line %d: tabs are not supported", lineNumber+1)
		}

		line := stripLegacyYAMLComment(strings.TrimRight(rawLine, " \r"))
		if strings.TrimSpace(line) == "" {
			continue
		}
		if strings.TrimSpace(line) == "---" || strings.TrimSpace(line) == "..." {
			continue
		}

		indent := countLeadingSpaces(line)
		if indent%2 != 0 {
			return parsedConfig, unsupportedServices, fmt.Errorf("line %d: indentation must use multiples of two spaces", lineNumber+1)
		}

		content := strings.TrimSpace(line)
		switch indent {
		case 0:
			key, value, ok := splitLegacyYAMLKeyValue(content)
			if !ok || value != "" {
				return parsedConfig, unsupportedServices, fmt.Errorf("line %d: expected top-level section", lineNumber+1)
			}
			if key != "services" && key != "system" {
				return parsedConfig, unsupportedServices, fmt.Errorf("line %d: unsupported top-level key %q", lineNumber+1, key)
			}
			section = key
			currentService = ""
			currentServiceIndex = -1

		case 2:
			switch section {
			case "services":
				key, value, ok := splitLegacyYAMLKeyValue(content)
				if !ok {
					return parsedConfig, unsupportedServices, fmt.Errorf("line %d: expected service key", lineNumber+1)
				}
				if value != "" && value != "[]" {
					return parsedConfig, unsupportedServices, fmt.Errorf("line %d: unsupported service value for %q", lineNumber+1, key)
				}
				if key == "organizr" {
					unsupportedServices = appendUniqueString(unsupportedServices, key)
					currentService = key
					currentServiceIndex = -1
					continue
				}
				if legacyServiceSlice(&parsedConfig, key) == nil {
					return parsedConfig, unsupportedServices, fmt.Errorf("line %d: unsupported service %q", lineNumber+1, key)
				}
				currentService = key
				currentServiceIndex = -1

			case "system":
				key, value, ok := splitLegacyYAMLKeyValue(content)
				if !ok {
					return parsedConfig, unsupportedServices, fmt.Errorf("line %d: expected system key", lineNumber+1)
				}
				switch key {
				case "compose_dir":
					parsedConfig.System.ComposeDir, err = parseLegacyYAMLString(value)
				case "tank_mount":
					parsedConfig.System.TankMount, err = parseLegacyYAMLString(value)
				case "network":
					if value != "" && value != "{}" {
						err = fmt.Errorf("unsupported network value %q", value)
					}
				default:
					err = fmt.Errorf("unsupported system key %q", key)
				}
				if err != nil {
					return parsedConfig, unsupportedServices, fmt.Errorf("line %d: %w", lineNumber+1, err)
				}

			default:
				return parsedConfig, unsupportedServices, fmt.Errorf("line %d: content outside services or system section", lineNumber+1)
			}

		case 4:
			switch section {
			case "services":
				if !strings.HasPrefix(content, "-") {
					return parsedConfig, unsupportedServices, fmt.Errorf("line %d: expected service list item", lineNumber+1)
				}
				if currentService == "" {
					return parsedConfig, unsupportedServices, fmt.Errorf("line %d: service list item outside service section", lineNumber+1)
				}
				if currentService == "organizr" {
					continue
				}

				itemContent := strings.TrimSpace(strings.TrimPrefix(content, "-"))
				currentServiceIndex = appendLegacyService(&parsedConfig, currentService)
				if itemContent == "" {
					continue
				}

				key, value, ok := splitLegacyYAMLKeyValue(itemContent)
				if !ok {
					return parsedConfig, unsupportedServices, fmt.Errorf("line %d: expected service field", lineNumber+1)
				}
				if err := setLegacyServiceField(&parsedConfig, currentService, currentServiceIndex, key, value); err != nil {
					return parsedConfig, unsupportedServices, fmt.Errorf("line %d: %w", lineNumber+1, err)
				}

			case "system":
				key, value, ok := splitLegacyYAMLKeyValue(content)
				if !ok || key != "interface" {
					return parsedConfig, unsupportedServices, fmt.Errorf("line %d: expected network interface", lineNumber+1)
				}
				interfaceName, err := parseLegacyYAMLString(value)
				if err != nil {
					return parsedConfig, unsupportedServices, fmt.Errorf("line %d: %w", lineNumber+1, err)
				}
				parsedConfig.System.Network.Interface = interfaceName

			default:
				return parsedConfig, unsupportedServices, fmt.Errorf("line %d: content outside services or system section", lineNumber+1)
			}

		case 6:
			if section != "services" {
				return parsedConfig, unsupportedServices, fmt.Errorf("line %d: unsupported nested value", lineNumber+1)
			}
			if currentService == "organizr" {
				continue
			}
			if currentService == "" || currentServiceIndex < 0 {
				return parsedConfig, unsupportedServices, fmt.Errorf("line %d: service field outside list item", lineNumber+1)
			}

			key, value, ok := splitLegacyYAMLKeyValue(content)
			if !ok {
				return parsedConfig, unsupportedServices, fmt.Errorf("line %d: expected service field", lineNumber+1)
			}
			if err := setLegacyServiceField(&parsedConfig, currentService, currentServiceIndex, key, value); err != nil {
				return parsedConfig, unsupportedServices, fmt.Errorf("line %d: %w", lineNumber+1, err)
			}

		default:
			return parsedConfig, unsupportedServices, fmt.Errorf("line %d: unsupported indentation level", lineNumber+1)
		}
	}

	return parsedConfig, unsupportedServices, nil
}

func newMigratedConfig() Config {
	var parsedConfig Config
	parsedConfig.Services.Plex = []ServiceConfig{}
	parsedConfig.Services.Jellyfin = []ServiceConfig{}
	parsedConfig.Services.Sonarr = []ServiceConfig{}
	parsedConfig.Services.Radarr = []ServiceConfig{}
	parsedConfig.Services.Seerr = []ServiceConfig{}
	return parsedConfig
}

func legacyServiceSlice(parsedConfig *Config, service string) *[]ServiceConfig {
	switch service {
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

func appendLegacyService(parsedConfig *Config, service string) int {
	services := legacyServiceSlice(parsedConfig, service)
	*services = append(*services, ServiceConfig{})
	return len(*services) - 1
}

func setLegacyServiceField(parsedConfig *Config, service string, index int, key, value string) error {
	services := legacyServiceSlice(parsedConfig, service)
	if services == nil || index < 0 || index >= len(*services) {
		return fmt.Errorf("service field outside list item")
	}

	serviceConfig := &(*services)[index]
	switch key {
	case "name":
		parsedValue, err := parseLegacyYAMLString(value)
		if err != nil {
			return err
		}
		serviceConfig.Name = parsedValue
	case "url":
		parsedValue, err := parseLegacyYAMLString(value)
		if err != nil {
			return err
		}
		serviceConfig.URL = parsedValue
	case "api_key":
		parsedValue, err := parseLegacyYAMLString(value)
		if err != nil {
			return err
		}
		serviceConfig.APIKey = parsedValue
	case "token":
		parsedValue, err := parseLegacyYAMLString(value)
		if err != nil {
			return err
		}
		serviceConfig.Token = parsedValue
	case "enabled":
		parsedValue, err := parseLegacyYAMLBool(value)
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
	inSingleQuote := false
	inDoubleQuote := false
	escaped := false

	for i := 0; i < len(line); i++ {
		current := line[i]
		if escaped {
			escaped = false
			continue
		}
		if inDoubleQuote && current == '\\' {
			escaped = true
			continue
		}

		switch current {
		case '\'':
			if !inDoubleQuote {
				inSingleQuote = !inSingleQuote
			}
		case '"':
			if !inSingleQuote {
				inDoubleQuote = !inDoubleQuote
			}
		case '#':
			if !inSingleQuote && !inDoubleQuote && (i == 0 || line[i-1] == ' ') {
				return strings.TrimRight(line[:i], " ")
			}
		}
	}

	return strings.TrimRight(line, " ")
}

func splitLegacyYAMLKeyValue(content string) (string, string, bool) {
	inSingleQuote := false
	inDoubleQuote := false
	escaped := false

	for i := 0; i < len(content); i++ {
		current := content[i]
		if escaped {
			escaped = false
			continue
		}
		if inDoubleQuote && current == '\\' {
			escaped = true
			continue
		}

		switch current {
		case '\'':
			if !inDoubleQuote {
				inSingleQuote = !inSingleQuote
			}
		case '"':
			if !inSingleQuote {
				inDoubleQuote = !inDoubleQuote
			}
		case ':':
			if !inSingleQuote && !inDoubleQuote {
				return strings.TrimSpace(content[:i]), strings.TrimSpace(content[i+1:]), true
			}
		}
	}

	return "", "", false
}

func parseLegacyYAMLString(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" || value == "null" || value == "~" {
		return "", nil
	}

	if strings.HasPrefix(value, "\"") {
		parsedValue, err := strconv.Unquote(value)
		if err != nil {
			return "", fmt.Errorf("invalid quoted string %q", value)
		}
		return parsedValue, nil
	}

	if strings.HasPrefix(value, "'") {
		if !strings.HasSuffix(value, "'") || len(value) == 1 {
			return "", fmt.Errorf("invalid quoted string %q", value)
		}
		return strings.ReplaceAll(value[1:len(value)-1], "''", "'"), nil
	}

	return value, nil
}

func parseLegacyYAMLBool(value string) (bool, error) {
	parsedValue, err := parseLegacyYAMLString(value)
	if err != nil {
		return false, err
	}

	switch strings.ToLower(parsedValue) {
	case "true", "yes", "on":
		return true, nil
	case "false", "no", "off":
		return false, nil
	default:
		return false, fmt.Errorf("invalid boolean %q", value)
	}
}

func countLeadingSpaces(value string) int {
	for i := 0; i < len(value); i++ {
		if value[i] != ' ' {
			return i
		}
	}

	return len(value)
}

func appendUniqueString(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}

	return append(values, value)
}
