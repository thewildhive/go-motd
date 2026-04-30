package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseLegacyYAMLConfig(t *testing.T) {
	legacyYAML := []byte(`services:
  plex:
    - name: "Main"
      url: "http://plex:32400"
      token: "plex-token"
      enabled: true
  jellyfin: []
  sonarr:
    - name: HD
      url: http://sonarr:8989
      api_key: sonarr-key
      enabled: yes
  radarr: []
  organizr:
    - name: Old
      url: http://organizr:80
      api_key: organizr-key
      enabled: true
system:
  compose_dir: "/opt/apps/compose"
  tank_mount: '/mnt/tank'
  network:
    interface: eth0
`)

	parsed, unsupported, err := parseLegacyYAMLConfig(legacyYAML)
	if err != nil {
		t.Fatalf("parseLegacyYAMLConfig failed: %v", err)
	}

	if len(parsed.Services.Plex) != 1 || parsed.Services.Plex[0].Token != "plex-token" {
		t.Fatalf("unexpected plex config: %+v", parsed.Services.Plex)
	}
	if len(parsed.Services.Jellyfin) != 0 {
		t.Fatalf("expected empty jellyfin config, got %+v", parsed.Services.Jellyfin)
	}
	if len(parsed.Services.Sonarr) != 1 || !parsed.Services.Sonarr[0].Enabled {
		t.Fatalf("unexpected sonarr config: %+v", parsed.Services.Sonarr)
	}
	if parsed.System.ComposeDir != "/opt/apps/compose" || parsed.System.TankMount != "/mnt/tank" {
		t.Fatalf("unexpected system config: %+v", parsed.System)
	}
	if parsed.System.Network.Interface != "eth0" {
		t.Fatalf("unexpected network interface: %s", parsed.System.Network.Interface)
	}
	if len(unsupported) != 1 || unsupported[0] != "organizr" {
		t.Fatalf("expected organizr to be reported unsupported, got %+v", unsupported)
	}

	encoded, err := json.Marshal(parsed)
	if err != nil {
		t.Fatalf("failed to encode migrated config: %v", err)
	}
	if _, err := decodeJSONConfig(encoded); err != nil {
		t.Fatalf("migrated config should decode as JSON config: %v", err)
	}
}

func TestParseLegacyYAMLConfig_UnsupportedShapeFails(t *testing.T) {
	_, _, err := parseLegacyYAMLConfig([]byte("services:\n  plex:\n    wrong: value\n"))
	if err == nil {
		t.Fatal("expected unsupported shape error")
	}
}

func TestMigrateLegacyConfigFromPaths_WritesJSON(t *testing.T) {
	tempDir := t.TempDir()
	legacyPath := filepath.Join(tempDir, "config.yml")
	jsonPath := filepath.Join(tempDir, "config.json")
	legacyYAML := []byte(`services:
  plex:
    - name: Main
      url: http://plex:32400
      token: plex-token
      enabled: true
  jellyfin: []
  sonarr: []
  radarr: []
  organizr: []
system:
  compose_dir: /opt/apps/compose
  tank_mount: /mnt/tank
`)
	if err := os.WriteFile(legacyPath, legacyYAML, 0o644); err != nil {
		t.Fatalf("failed to write legacy config: %v", err)
	}

	actualLegacyPath, actualJSONPath, unsupported, err := migrateLegacyConfigFromPaths([]string{jsonPath}, []string{legacyPath})
	if err != nil {
		t.Fatalf("migrateLegacyConfigFromPaths failed: %v", err)
	}
	if actualLegacyPath != legacyPath || actualJSONPath != jsonPath {
		t.Fatalf("unexpected migration paths: %s -> %s", actualLegacyPath, actualJSONPath)
	}
	if len(unsupported) != 1 || unsupported[0] != "organizr" {
		t.Fatalf("expected organizr unsupported warning, got %+v", unsupported)
	}

	data, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatalf("failed to read migrated config: %v", err)
	}
	parsed, err := decodeJSONConfig(data)
	if err != nil {
		t.Fatalf("migrated JSON should decode: %v", err)
	}
	if len(parsed.Services.Plex) != 1 || parsed.Services.Plex[0].URL != "http://plex:32400" {
		t.Fatalf("unexpected migrated plex config: %+v", parsed.Services.Plex)
	}
	if parsed.System.TankMount != "/mnt/tank" {
		t.Fatalf("unexpected migrated tank mount: %s", parsed.System.TankMount)
	}
}

func TestMigrateLegacyConfigFromPaths_ExistingJSONFails(t *testing.T) {
	tempDir := t.TempDir()
	legacyPath := filepath.Join(tempDir, "config.yml")
	jsonPath := filepath.Join(tempDir, "config.json")
	if err := os.WriteFile(legacyPath, []byte("services: {}\n"), 0o644); err != nil {
		t.Fatalf("failed to write legacy config: %v", err)
	}
	if err := os.WriteFile(jsonPath, []byte("{}"), 0o644); err != nil {
		t.Fatalf("failed to write JSON config: %v", err)
	}

	_, _, _, err := migrateLegacyConfigFromPaths([]string{jsonPath}, []string{legacyPath})
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("expected existing JSON error, got %v", err)
	}
}

func TestMigrateLegacyConfigFromPaths_MissingLegacyReturnsSentinel(t *testing.T) {
	tempDir := t.TempDir()
	_, _, _, err := migrateLegacyConfigFromPaths(
		[]string{filepath.Join(tempDir, "config.json")},
		[]string{filepath.Join(tempDir, "config.yml")},
	)
	if !errors.Is(err, errNoLegacyConfig) {
		t.Fatalf("expected errNoLegacyConfig, got %v", err)
	}
}
