package config

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

var ErrNoJSONConfig = errors.New("no JSON config files found")

type LegacyConfigError struct {
	LegacyPath   string
	RequiredPath string
	FallbackPath string
}

func (e *LegacyConfigError) Error() string {
	return fmt.Sprintf("legacy YAML config detected at %s", e.LegacyPath)
}

func GetConfigPaths() []string {
	home := getUserHome()
	userConfig := filepath.Join(home, ".config", "motd", "config.json")
	if home == "" {
		return []string{"/opt/motd/config.json"}
	}
	return []string{userConfig, "/opt/motd/config.json"}
}

func GetLegacyConfigPaths() []string {
	home := getUserHome()
	userYML := filepath.Join(home, ".config", "motd", "config.yml")
	userYAML := filepath.Join(home, ".config", "motd", "config.yaml")
	if home == "" {
		return []string{"/opt/motd/config.yml", "/opt/motd/config.yaml"}
	}
	return []string{userYML, userYAML, "/opt/motd/config.yml", "/opt/motd/config.yaml"}
}

func GetExplicitLegacyConfigPaths(configPath string) []string {
	dir := filepath.Dir(configPath)
	return []string{filepath.Join(dir, "config.yml"), filepath.Join(dir, "config.yaml")}
}

func DecodeJSONConfig(data []byte) (Config, error) {
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

func LoadJSONConfigFromPaths(paths []string, debugFn func(string, ...interface{})) (Config, error) {
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

		if debugFn != nil {
			debugFn("Loading JSON config from: %s", configPath)
		}
		data, err := os.ReadFile(configPath)
		if err != nil {
			return loadedConfig, fmt.Errorf("failed to read config file %s: %w", configPath, err)
		}

		parsedConfig, err := DecodeJSONConfig(data)
		if err != nil {
			return loadedConfig, fmt.Errorf("failed to parse JSON config %s: %w", configPath, err)
		}

		if debugFn != nil {
			debugFn("Successfully loaded JSON config from: %s", configPath)
		}
		return parsedConfig, nil
	}

	if debugFn != nil {
		debugFn("No JSON config files found")
	}
	return loadedConfig, ErrNoJSONConfig
}

func DetectLegacyYAMLConfig(legacyPaths, jsonPaths []string) *LegacyConfigError {
	fallbackPath := ""
	if len(jsonPaths) > 1 {
		fallbackPath = jsonPaths[1]
	}

	for i, legacyPath := range legacyPaths {
		if _, err := os.Stat(legacyPath); err == nil {
			requiredPath := matchingJSONConfigPath(legacyPath, jsonPaths, i)

			return &LegacyConfigError{
				LegacyPath:   legacyPath,
				RequiredPath: requiredPath,
				FallbackPath: fallbackPath,
			}
		}
	}

	return nil
}

func matchingJSONConfigPath(legacyPath string, jsonPaths []string, legacyIndex int) string {
	for _, jsonPath := range jsonPaths {
		if filepath.Dir(jsonPath) == filepath.Dir(legacyPath) {
			return jsonPath
		}
	}

	if legacyIndex < len(jsonPaths) {
		return jsonPaths[legacyIndex]
	}
	if len(jsonPaths) > 0 {
		return jsonPaths[0]
	}

	return ""
}

func LoadFromPaths(jsonPaths, legacyPaths []string, debugFn func(string, ...interface{})) (Config, error) {
	loadedConfig, err := LoadJSONConfigFromPaths(jsonPaths, debugFn)
	if err == nil {
		return loadedConfig, nil
	}

	if errors.Is(err, ErrNoJSONConfig) {
		if legacyErr := DetectLegacyYAMLConfig(legacyPaths, jsonPaths); legacyErr != nil {
			return Config{}, legacyErr
		}

		if debugFn != nil {
			debugFn("No JSON configuration found; continuing with system-only defaults")
		}
		return Config{}, nil
	}

	return Config{}, err
}

func Load(configPath string, noConfig bool, debugFn func(string, ...interface{})) (Config, error) {
	if noConfig {
		if debugFn != nil {
			debugFn("Config loading skipped by -no-config")
		}
		return Config{}, nil
	}

	if strings.TrimSpace(configPath) != "" {
		return LoadFromPaths([]string{configPath}, GetExplicitLegacyConfigPaths(configPath), debugFn)
	}

	return LoadFromPaths(GetConfigPaths(), GetLegacyConfigPaths(), debugFn)
}

func PrintLegacyConfigError(err *LegacyConfigError) {
	fmt.Printf("%sError: Legacy YAML config is no longer supported.%s\n", "\033[0;31m", "\033[0m")
	fmt.Printf("Found legacy config at: %s\n", err.LegacyPath)
	if err.RequiredPath != "" {
		fmt.Printf("Create a JSON config at: %s\n", err.RequiredPath)
		fmt.Printf("Or migrate automatically with: motd -config %s -migrate\n", err.RequiredPath)
	}
	if err.FallbackPath != "" {
		fmt.Printf("Fallback JSON path: %s\n", err.FallbackPath)
	}
}

// Write saves cfg as pretty-printed JSON to path, creating parent
// directories as needed. Returns an error if marshalling, directory
// creation, or file writing fails.
func Write(path string, cfg Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}
	data = append(data, '\n')

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}

func getUserHome() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return home
}
