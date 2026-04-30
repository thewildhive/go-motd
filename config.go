package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type ServiceConfig struct {
	Name    string `json:"name"`
	URL     string `json:"url"`
	APIKey  string `json:"api_key,omitempty"`
	Token   string `json:"token,omitempty"`
	Enabled bool   `json:"enabled"`
}

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

func getExplicitLegacyConfigPaths(configPath string) []string {
	dir := filepath.Dir(configPath)
	return []string{filepath.Join(dir, "config.yml"), filepath.Join(dir, "config.yaml")}
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

		debugLog("No JSON configuration found; continuing with system-only defaults")
		return Config{}, nil
	}

	return Config{}, err
}

func loadRuntimeConfig(configPath string, noConfig bool) (Config, error) {
	if noConfig {
		debugLog("Config loading skipped by -no-config")
		return Config{}, nil
	}

	if strings.TrimSpace(configPath) != "" {
		return loadRuntimeConfigFromPaths([]string{configPath}, getExplicitLegacyConfigPaths(configPath))
	}

	return loadRuntimeConfigFromPaths(getConfigPaths(), getLegacyConfigPaths())
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

func loadConfig(configPath string, noConfig bool) {
	loadedConfig, err := loadRuntimeConfig(configPath, noConfig)
	if err != nil {
		var legacyErr *legacyConfigError
		if errors.As(err, &legacyErr) {
			printLegacyConfigError(legacyErr)
			os.Exit(1)
		}

		fmt.Printf("%sError loading configuration: %v%s\n", RED, err, RESET)
		os.Exit(1)
	}

	config = loadedConfig
	if noConfig {
		debugLog("Using system-only defaults")
	} else {
		debugLog("Runtime configuration ready")
	}
}
