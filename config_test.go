package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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

	parsed, err := decodeJSONConfig(configJSON)
	if err != nil {
		t.Fatalf("decodeJSONConfig failed: %v", err)
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

	_, err := decodeJSONConfig(configJSON)
	if err == nil {
		t.Fatal("expected unknown field error, got nil")
	}

	if !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("expected unknown field error, got: %v", err)
	}
}

func TestDecodeJSONConfig_MultipleObjectsFail(t *testing.T) {
	configJSON := []byte(`{"services":{"plex":[],"jellyfin":[],"sonarr":[],"radarr":[],"seerr":[]},"system":{}} {}`)

	_, err := decodeJSONConfig(configJSON)
	if err == nil {
		t.Fatal("expected error for multiple JSON objects")
	}
}

func TestLoadJSONConfigFromPaths_ParsesFile(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")

	configJSON := []byte(`{"services":{"plex":[],"jellyfin":[],"sonarr":[],"radarr":[],"seerr":[]},"system":{"compose_dir":"/opt/apps/compose","tank_mount":"/mnt/tank"}}`)
	if err := os.WriteFile(configPath, configJSON, 0o644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	loaded, err := loadJSONConfigFromPaths([]string{configPath})
	if err != nil {
		t.Fatalf("loadJSONConfigFromPaths failed: %v", err)
	}

	if loaded.System.ComposeDir != "/opt/apps/compose" {
		t.Fatalf("unexpected compose_dir: %s", loaded.System.ComposeDir)
	}
}

func TestLoadJSONConfigFromPaths_MissingReturnsSentinel(t *testing.T) {
	_, err := loadJSONConfigFromPaths([]string{"/path/that/does/not/exist/config.json"})
	if !errors.Is(err, errNoJSONConfig) {
		t.Fatalf("expected errNoJSONConfig, got: %v", err)
	}
}

func TestLoadRuntimeConfigFromPaths_MissingUsesZeroConfig(t *testing.T) {
	loaded, err := loadRuntimeConfigFromPaths([]string{"/path/that/does/not/exist/config.json"}, nil)
	if err != nil {
		t.Fatalf("expected missing config to use defaults, got: %v", err)
	}

	if len(loaded.Services.Plex) != 0 || loaded.System.ComposeDir != "" {
		t.Fatalf("expected zero-value config, got: %+v", loaded)
	}
}

func TestHasMediaServicesRequiresURLAndCredentials(t *testing.T) {
	originalConfig := config
	t.Cleanup(func() {
		config = originalConfig
	})

	tests := []struct {
		name        string
		missingURL  ServiceConfig
		missingAuth ServiceConfig
		ready       ServiceConfig
		disabled    ServiceConfig
		apply       func(ServiceConfig)
	}{
		{
			name:        "plex",
			missingURL:  ServiceConfig{Enabled: true, Token: "secret"},
			missingAuth: ServiceConfig{URL: "http://plex:32400", Enabled: true},
			ready:       ServiceConfig{URL: "http://plex:32400", Token: "secret", Enabled: true},
			disabled:    ServiceConfig{URL: "http://plex:32400", Token: "secret", Enabled: false},
			apply:       func(service ServiceConfig) { config.Services.Plex = []ServiceConfig{service} },
		},
		{
			name:        "jellyfin",
			missingURL:  ServiceConfig{Enabled: true, Token: "secret"},
			missingAuth: ServiceConfig{URL: "http://jellyfin:8096", Enabled: true},
			ready:       ServiceConfig{URL: "http://jellyfin:8096", Token: "secret", Enabled: true},
			disabled:    ServiceConfig{URL: "http://jellyfin:8096", Token: "secret", Enabled: false},
			apply:       func(service ServiceConfig) { config.Services.Jellyfin = []ServiceConfig{service} },
		},
		{
			name:        "sonarr",
			missingURL:  ServiceConfig{Enabled: true, APIKey: "secret"},
			missingAuth: ServiceConfig{URL: "http://sonarr:8989", Enabled: true},
			ready:       ServiceConfig{URL: "http://sonarr:8989", APIKey: "secret", Enabled: true},
			disabled:    ServiceConfig{URL: "http://sonarr:8989", APIKey: "secret", Enabled: false},
			apply:       func(service ServiceConfig) { config.Services.Sonarr = []ServiceConfig{service} },
		},
		{
			name:        "radarr",
			missingURL:  ServiceConfig{Enabled: true, APIKey: "secret"},
			missingAuth: ServiceConfig{URL: "http://radarr:7878", Enabled: true},
			ready:       ServiceConfig{URL: "http://radarr:7878", APIKey: "secret", Enabled: true},
			disabled:    ServiceConfig{URL: "http://radarr:7878", APIKey: "secret", Enabled: false},
			apply:       func(service ServiceConfig) { config.Services.Radarr = []ServiceConfig{service} },
		},
		{
			name:        "seerr",
			missingURL:  ServiceConfig{Enabled: true, APIKey: "secret"},
			missingAuth: ServiceConfig{URL: "http://seerr:5055", Enabled: true},
			ready:       ServiceConfig{URL: "http://seerr:5055", APIKey: "secret", Enabled: true},
			disabled:    ServiceConfig{URL: "http://seerr:5055", APIKey: "secret", Enabled: false},
			apply:       func(service ServiceConfig) { config.Services.Seerr = []ServiceConfig{service} },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config = Config{}
			tt.apply(tt.missingURL)
			if hasMediaServices() {
				t.Fatal("expected media services to be disabled without URL")
			}

			config = Config{}
			tt.apply(tt.missingAuth)
			if hasMediaServices() {
				t.Fatal("expected media services to be disabled without credentials")
			}

			config = Config{}
			tt.apply(tt.disabled)
			if hasMediaServices() {
				t.Fatal("expected media services to be disabled when service is disabled")
			}

			config = Config{}
			tt.apply(tt.ready)
			if !hasMediaServices() {
				t.Fatal("expected media services to be enabled with URL and credentials")
			}
		})
	}
}

func TestLoadJSONConfigFromPaths_PriorityAndFallback(t *testing.T) {
	tempDir := t.TempDir()
	firstPath := filepath.Join(tempDir, "first.json")
	secondPath := filepath.Join(tempDir, "second.json")

	firstJSON := []byte(`{"services":{"plex":[],"jellyfin":[],"sonarr":[],"radarr":[],"seerr":[]},"system":{"compose_dir":"/first"}}`)
	secondJSON := []byte(`{"services":{"plex":[],"jellyfin":[],"sonarr":[],"radarr":[],"seerr":[]},"system":{"compose_dir":"/second"}}`)
	if err := os.WriteFile(firstPath, firstJSON, 0o644); err != nil {
		t.Fatalf("failed to write first config: %v", err)
	}
	if err := os.WriteFile(secondPath, secondJSON, 0o644); err != nil {
		t.Fatalf("failed to write second config: %v", err)
	}

	loaded, err := loadJSONConfigFromPaths([]string{firstPath, secondPath})
	if err != nil {
		t.Fatalf("loadJSONConfigFromPaths failed: %v", err)
	}
	if loaded.System.ComposeDir != "/first" {
		t.Fatalf("expected first config to win, got %q", loaded.System.ComposeDir)
	}

	loaded, err = loadJSONConfigFromPaths([]string{filepath.Join(tempDir, "missing.json"), secondPath})
	if err != nil {
		t.Fatalf("loadJSONConfigFromPaths fallback failed: %v", err)
	}
	if loaded.System.ComposeDir != "/second" {
		t.Fatalf("expected second config fallback, got %q", loaded.System.ComposeDir)
	}
}

func TestLoadJSONConfigFromPaths_DirectoryErrors(t *testing.T) {
	_, err := loadJSONConfigFromPaths([]string{t.TempDir()})
	if err == nil || !strings.Contains(err.Error(), "directory") {
		t.Fatalf("expected directory error, got %v", err)
	}
}

func TestLoadRuntimeConfig_ExplicitConfigDetectsSiblingYAML(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "custom.json")
	legacyPath := filepath.Join(tempDir, "config.yaml")
	if err := os.WriteFile(legacyPath, []byte("services: {}"), 0o644); err != nil {
		t.Fatalf("failed to write sibling legacy config: %v", err)
	}

	_, err := loadRuntimeConfig(configPath, false)
	if err == nil {
		t.Fatal("expected sibling legacy config error")
	}
	var legacyErr *legacyConfigError
	if !errors.As(err, &legacyErr) || legacyErr.legacyPath != legacyPath {
		t.Fatalf("expected sibling legacy error for %s, got %v", legacyPath, err)
	}
}

func TestLoadRuntimeConfigFromPaths_LegacyYAMLDetected(t *testing.T) {
	tempDir := t.TempDir()
	legacyPath := filepath.Join(tempDir, "config.yml")
	if err := os.WriteFile(legacyPath, []byte("services: {}"), 0o644); err != nil {
		t.Fatalf("failed to create legacy config: %v", err)
	}

	_, err := loadRuntimeConfigFromPaths(
		[]string{filepath.Join(tempDir, "missing.json")},
		[]string{legacyPath},
	)
	if err == nil {
		t.Fatal("expected error when only legacy config exists")
	}

	var legacyErr *legacyConfigError
	if !errors.As(err, &legacyErr) {
		t.Fatalf("expected legacyConfigError, got: %v", err)
	}
}

func TestLoadRuntimeConfig_ExplicitConfig(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "custom.json")

	configJSON := []byte(`{"services":{"plex":[],"jellyfin":[],"sonarr":[],"radarr":[],"seerr":[]},"system":{"compose_dir":"/custom/compose","tank_mount":"/mnt/tank"}}`)
	if err := os.WriteFile(configPath, configJSON, 0o644); err != nil {
		t.Fatalf("failed to write explicit config: %v", err)
	}

	loaded, err := loadRuntimeConfig(configPath, false)
	if err != nil {
		t.Fatalf("loadRuntimeConfig failed: %v", err)
	}

	if loaded.System.ComposeDir != "/custom/compose" {
		t.Fatalf("unexpected compose_dir: %s", loaded.System.ComposeDir)
	}
}

func TestLoadRuntimeConfig_NoConfigSkipsInvalidConfigPath(t *testing.T) {
	loaded, err := loadRuntimeConfig("/path/that/does/not/exist/config.json", true)
	if err != nil {
		t.Fatalf("expected -no-config to skip loading, got: %v", err)
	}

	if len(loaded.Services.Plex) != 0 || loaded.System.TankMount != "" {
		t.Fatalf("expected zero-value config, got: %+v", loaded)
	}
}

func TestDetectLegacyYAMLConfigFromPaths(t *testing.T) {
	tempDir := t.TempDir()
	legacyPath := filepath.Join(tempDir, "config.yml")
	if err := os.WriteFile(legacyPath, []byte("services: {}"), 0o644); err != nil {
		t.Fatalf("failed to create legacy config: %v", err)
	}

	err := detectLegacyYAMLConfigFromPaths([]string{legacyPath}, []string{"/new/config.json", "/fallback/config.json"})
	if err == nil {
		t.Fatal("expected legacy config error")
	}

	if err.legacyPath != legacyPath {
		t.Fatalf("unexpected legacy path: %s", err.legacyPath)
	}
	if err.requiredPath != "/new/config.json" {
		t.Fatalf("unexpected required path: %s", err.requiredPath)
	}
}

func TestDetectLegacyYAMLConfigFromPaths_MatchesJSONDirectory(t *testing.T) {
	tempDir := t.TempDir()
	userDir := filepath.Join(tempDir, "user")
	fallbackDir := filepath.Join(tempDir, "fallback")
	if err := os.MkdirAll(userDir, 0o755); err != nil {
		t.Fatalf("failed to create user dir: %v", err)
	}
	if err := os.MkdirAll(fallbackDir, 0o755); err != nil {
		t.Fatalf("failed to create fallback dir: %v", err)
	}

	legacyPath := filepath.Join(userDir, "config.yaml")
	if err := os.WriteFile(legacyPath, []byte("services: {}"), 0o644); err != nil {
		t.Fatalf("failed to create legacy config: %v", err)
	}

	userJSONPath := filepath.Join(userDir, "config.json")
	fallbackJSONPath := filepath.Join(fallbackDir, "config.json")
	legacyErr := detectLegacyYAMLConfigFromPaths(
		[]string{filepath.Join(userDir, "config.yml"), legacyPath, filepath.Join(fallbackDir, "config.yml"), filepath.Join(fallbackDir, "config.yaml")},
		[]string{userJSONPath, fallbackJSONPath},
	)
	if legacyErr == nil {
		t.Fatal("expected legacy config error")
	}
	if legacyErr.requiredPath != userJSONPath {
		t.Fatalf("expected matching user JSON path, got %s", legacyErr.requiredPath)
	}
}
