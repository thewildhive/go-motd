package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func noDebug(_ string, _ ...interface{}) {}

func TestDecodeJSONConfig_Success(t *testing.T) {
	configJSON := []byte(`{
		"services": {
			"plex": [],
			"jellyfin": [],
			"sonarr": [],
			"radarr": [],
			"seerr": [
				{
					"name": "Main",
					"url": "http://seerr:5055",
					"api_key": "abc",
					"enabled": true
				}
			]
		},
		"system": {
			"compose_dir": "/opt/apps/compose",
			"tank_mount": "/mnt/tank",
			"network": {"interface": "eth0"}
		}
	}`)

	parsed, err := DecodeJSONConfig(configJSON)
	if err != nil {
		t.Fatalf("DecodeJSONConfig failed: %v", err)
	}

	if len(parsed.Services.Seerr) != 1 {
		t.Fatalf("expected 1 seerr service, got %d", len(parsed.Services.Seerr))
	}

	if parsed.Services.Seerr[0].APIKey != "abc" {
		t.Fatalf("unexpected seerr api key: %s", parsed.Services.Seerr[0].APIKey)
	}
}

func TestDecodeJSONConfig_UnknownFieldFails(t *testing.T) {
	configJSON := []byte(`{"services":{"plex":[],"jellyfin":[],"sonarr":[],"radarr":[],"seerr":[],"organizr":[]},"system":{"compose_dir":"/opt/apps/compose","tank_mount":"/mnt/tank"}}`)

	_, err := DecodeJSONConfig(configJSON)
	if err == nil {
		t.Fatal("expected unknown field error, got nil")
	}

	if !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("expected unknown field error, got: %v", err)
	}
}

func TestDecodeJSONConfig_MultipleObjectsFail(t *testing.T) {
	configJSON := []byte(`{"services":{"plex":[],"jellyfin":[],"sonarr":[],"radarr":[],"seerr":[]},"system":{}} {}`)

	_, err := DecodeJSONConfig(configJSON)
	if err == nil {
		t.Fatal("expected error for multiple JSON objects")
	}
}

func TestLoadJSONConfigFromPaths_ParsesFile(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")

	configJSON := []byte(`{"services":{"plex":[],"jellyfin":[],"sonarr":[],"radarr":[],"seerr":[]},"system":{"compose_dir":"/opt/apps/compose","tank_mount":"/mnt/tank"}}`)
	if err := os.WriteFile(configPath, configJSON, 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	loaded, err := LoadJSONConfigFromPaths([]string{configPath}, noDebug)
	if err != nil {
		t.Fatalf("LoadJSONConfigFromPaths failed: %v", err)
	}

	if loaded.System.ComposeDir != "/opt/apps/compose" {
		t.Fatalf("unexpected compose_dir: %s", loaded.System.ComposeDir)
	}
}

func TestLoadJSONConfigFromPaths_MissingReturnsSentinel(t *testing.T) {
	_, err := LoadJSONConfigFromPaths([]string{"/path/that/does/not/exist/config.json"}, noDebug)
	if !errors.Is(err, ErrNoJSONConfig) {
		t.Fatalf("expected ErrNoJSONConfig, got: %v", err)
	}
}

func TestLoadFromPaths_MissingUsesZeroConfig(t *testing.T) {
	loaded, err := LoadFromPaths([]string{"/path/that/does/not/exist/config.json"}, nil, noDebug)
	if err != nil {
		t.Fatalf("expected missing config to use defaults, got: %v", err)
	}

	if len(loaded.Services.Plex) != 0 || loaded.System.ComposeDir != "" {
		t.Fatalf("expected zero-value config, got: %+v", loaded)
	}
}

func TestLoadJSONConfigFromPaths_PriorityAndFallback(t *testing.T) {
	tempDir := t.TempDir()
	firstPath := filepath.Join(tempDir, "first.json")
	secondPath := filepath.Join(tempDir, "second.json")

	firstJSON := []byte(`{"services":{"plex":[],"jellyfin":[],"sonarr":[],"radarr":[],"seerr":[]},"system":{"compose_dir":"/first"}}`)
	secondJSON := []byte(`{"services":{"plex":[],"jellyfin":[],"sonarr":[],"radarr":[],"seerr":[]},"system":{"compose_dir":"/second"}}`)
	if err := os.WriteFile(firstPath, firstJSON, 0644); err != nil {
		t.Fatalf("failed to write first config: %v", err)
	}
	if err := os.WriteFile(secondPath, secondJSON, 0644); err != nil {
		t.Fatalf("failed to write second config: %v", err)
	}

	loaded, err := LoadJSONConfigFromPaths([]string{firstPath, secondPath}, noDebug)
	if err != nil {
		t.Fatalf("LoadJSONConfigFromPaths failed: %v", err)
	}
	if loaded.System.ComposeDir != "/first" {
		t.Fatalf("expected first config to win, got %q", loaded.System.ComposeDir)
	}

	loaded, err = LoadJSONConfigFromPaths([]string{filepath.Join(tempDir, "missing.json"), secondPath}, noDebug)
	if err != nil {
		t.Fatalf("LoadJSONConfigFromPaths fallback failed: %v", err)
	}
	if loaded.System.ComposeDir != "/second" {
		t.Fatalf("expected second config fallback, got %q", loaded.System.ComposeDir)
	}
}

func TestLoadJSONConfigFromPaths_DirectoryErrors(t *testing.T) {
	_, err := LoadJSONConfigFromPaths([]string{t.TempDir()}, noDebug)
	if err == nil || !strings.Contains(err.Error(), "directory") {
		t.Fatalf("expected directory error, got %v", err)
	}
}

func TestLoad_ExplicitConfigDetectsSiblingYAML(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "custom.json")
	legacyPath := filepath.Join(tempDir, "config.yaml")
	if err := os.WriteFile(legacyPath, []byte("services: {}"), 0644); err != nil {
		t.Fatalf("failed to write sibling legacy config: %v", err)
	}

	_, err := Load(configPath, false, noDebug)
	if err == nil {
		t.Fatal("expected sibling legacy config error")
	}
	var legacyErr *LegacyConfigError
	if !errors.As(err, &legacyErr) || legacyErr.LegacyPath != legacyPath {
		t.Fatalf("expected sibling legacy error for %s, got %v", legacyPath, err)
	}
}

func TestLoadFromPaths_LegacyYAMLDetected(t *testing.T) {
	tempDir := t.TempDir()
	legacyPath := filepath.Join(tempDir, "config.yml")
	if err := os.WriteFile(legacyPath, []byte("services: {}"), 0644); err != nil {
		t.Fatalf("failed to create legacy config: %v", err)
	}

	_, err := LoadFromPaths(
		[]string{filepath.Join(tempDir, "missing.json")},
		[]string{legacyPath},
		noDebug,
	)
	if err == nil {
		t.Fatal("expected error when only legacy config exists")
	}

	var legacyErr *LegacyConfigError
	if !errors.As(err, &legacyErr) {
		t.Fatalf("expected LegacyConfigError, got: %v", err)
	}
}

func TestLoad_ExplicitConfig(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "custom.json")

	configJSON := []byte(`{"services":{"plex":[],"jellyfin":[],"sonarr":[],"radarr":[],"seerr":[]},"system":{"compose_dir":"/custom/compose","tank_mount":"/mnt/tank"}}`)
	if err := os.WriteFile(configPath, configJSON, 0644); err != nil {
		t.Fatalf("failed to write explicit config: %v", err)
	}

	loaded, err := Load(configPath, false, noDebug)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if loaded.System.ComposeDir != "/custom/compose" {
		t.Fatalf("unexpected compose_dir: %s", loaded.System.ComposeDir)
	}
}

func TestLoad_NoConfigSkipsInvalidConfigPath(t *testing.T) {
	loaded, err := Load("/path/that/does/not/exist/config.json", true, noDebug)
	if err != nil {
		t.Fatalf("expected -no-config to skip loading, got: %v", err)
	}

	if len(loaded.Services.Plex) != 0 || loaded.System.TankMount != "" {
		t.Fatalf("expected zero-value config, got: %+v", loaded)
	}
}

func TestDetectLegacyYAML(t *testing.T) {
	tempDir := t.TempDir()
	legacyPath := filepath.Join(tempDir, "config.yml")
	if err := os.WriteFile(legacyPath, []byte("services: {}"), 0644); err != nil {
		t.Fatalf("failed to create legacy config: %v", err)
	}

	legacyErr := DetectLegacyYAMLConfig([]string{legacyPath}, []string{"/new/config.json", "/fallback/config.json"})
	if legacyErr == nil {
		t.Fatal("expected legacy config error")
	}

	if legacyErr.LegacyPath != legacyPath {
		t.Fatalf("unexpected legacy path: %s", legacyErr.LegacyPath)
	}
	if legacyErr.RequiredPath != "/new/config.json" {
		t.Fatalf("unexpected required path: %s", legacyErr.RequiredPath)
	}
}

func TestDetectLegacyYAML_MatchesJSONDirectory(t *testing.T) {
	tempDir := t.TempDir()
	userDir := filepath.Join(tempDir, "user")
	fallbackDir := filepath.Join(tempDir, "fallback")
	if err := os.MkdirAll(userDir, 0755); err != nil {
		t.Fatalf("failed to create user dir: %v", err)
	}
	if err := os.MkdirAll(fallbackDir, 0755); err != nil {
		t.Fatalf("failed to create fallback dir: %v", err)
	}

	legacyPath := filepath.Join(userDir, "config.yaml")
	if err := os.WriteFile(legacyPath, []byte("services: {}"), 0644); err != nil {
		t.Fatalf("failed to create legacy config: %v", err)
	}

	userJSONPath := filepath.Join(userDir, "config.json")
	fallbackJSONPath := filepath.Join(fallbackDir, "config.json")
	legacyErr := DetectLegacyYAMLConfig(
		[]string{filepath.Join(userDir, "config.yml"), legacyPath, filepath.Join(fallbackDir, "config.yml"), filepath.Join(fallbackDir, "config.yaml")},
		[]string{userJSONPath, fallbackJSONPath},
	)
	if legacyErr == nil {
		t.Fatal("expected legacy config error")
	}
	if legacyErr.RequiredPath != userJSONPath {
		t.Fatalf("expected matching user JSON path, got %s", legacyErr.RequiredPath)
	}
}
