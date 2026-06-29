package config

import (
	"os"
	"path/filepath"
	"runtime"
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
			"compose_dir": "/opt/compose",
			"tank_mount": "/mnt/tank"
		}
	}
	`)
	cfg, err := DecodeJSONConfig(configJSON)
	if err != nil {
		t.Fatalf("DecodeJSONConfig failed: %v", err)
	}
	if cfg.System.ComposeDir != "/opt/compose" || cfg.System.TankMount != "/mnt/tank" {
		t.Fatalf("unexpected config: %+v", cfg.System)
	}
	if len(cfg.Services.Seerr) != 1 {
		t.Fatalf("expected one seerr service, got %d", len(cfg.Services.Seerr))
	}
}

func TestDecodeJSONConfig_UnknownField(t *testing.T) {
	configJSON := []byte(`{
		"services": {
			"plex": []
		},
		"system": {
			"compose_dir": "/opt/compose"
		},
		"extra": true
	}
	`)
	_, err := DecodeJSONConfig(configJSON)
	if err == nil {
		t.Fatal("expected unknown field to fail")
	}
}

func TestDecodeJSONConfig_MultipleObjects(t *testing.T) {
	configJSON := []byte(`{"services": {}} {"services": {}}`)
	_, err := DecodeJSONConfig(configJSON)
	if err == nil {
		t.Fatal("expected multiple JSON objects to fail")
	}
}

func TestLoadFromPaths_PrefersJSON(t *testing.T) {
	jsonPath := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(jsonPath, []byte(`{"services": {}}`), 0644); err != nil {
		t.Fatalf("failed to write json config: %v", err)
	}

	legacyYml := filepath.Join(filepath.Dir(jsonPath), "config.yml")
	if err := os.WriteFile(legacyYml, []byte("services: {}"), 0644); err != nil {
		t.Fatalf("failed to write legacy file: %v", err)
	}

	cfg, err := LoadFromPaths([]string{jsonPath}, []string{legacyYml}, nil)
	if err != nil {
		t.Fatalf("LoadFromPaths failed: %v", err)
	}
	if cfg.Services.Plex != nil {
		t.Fatalf("expected loaded config services")
	}
}

func TestLoadFromPaths_LegacyDetected(t *testing.T) {
	tempDir := t.TempDir()
	legacy := filepath.Join(tempDir, "config.yml")
	if err := os.WriteFile(legacy, []byte("services: {}"), 0644); err != nil {
		t.Fatalf("failed to write legacy config: %v", err)
	}

	jsonPath := filepath.Join(tempDir, "config.json")
	_, err := LoadFromPaths([]string{jsonPath}, []string{legacy}, nil)
	if err == nil {
		t.Fatal("expected legacy detection")
	}
	legacyErr, ok := err.(*LegacyConfigError)
	if !ok {
		t.Fatalf("expected LegacyConfigError, got %T", err)
	}
	if legacyErr.RequiredPath != jsonPath {
		t.Fatalf("expected required path %s, got %s", jsonPath, legacyErr.RequiredPath)
	}
}

func TestLoadFromPaths_NoConfigReturnsEmpty(t *testing.T) {
	cfg, err := LoadFromPaths([]string{filepath.Join(t.TempDir(), "missing.json")}, nil, noDebug)
	if err != nil {
		t.Fatalf("expected no config and no error, got %v", err)
	}
	if cfg.Services.Plex != nil || cfg.System.ComposeDir != "" {
		t.Fatalf("expected empty config")
	}
}

func TestLoad_MissingExplicitPath(t *testing.T) {
	cfg, err := Load("/some/missing/path/config.json", false, noDebug)
	if err != nil {
		t.Fatalf("expected missing explicit path to be acceptable with migration hint behavior, got %v", err)
	}
	if cfg.Services.Plex != nil {
		t.Fatalf("expected empty config")
	}
}

func TestLoad_NoConfigSkips(t *testing.T) {
	cfg, err := Load("", true, noDebug)
	if err != nil {
		t.Fatalf("Load should skip config with no-config: %v", err)
	}
	if cfg.Services.Plex != nil {
		t.Fatalf("expected no services")
	}
}

func TestLoadJSONConfigFromPaths_ReturnsErrOnUnreadableFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nope.json")
	if err := os.WriteFile(path, []byte("{"), 0644); err != nil {
		t.Fatalf("failed to write bad json: %v", err)
	}

	_, err := LoadJSONConfigFromPaths([]string{path}, nil)
	if err == nil {
		t.Fatal("expected parse failure")
	}
}

func TestLoadJSONConfigFromPaths_ParsesFirstValid(t *testing.T) {
	dir := t.TempDir()
	path1 := filepath.Join(dir, "missing.json")
	path2 := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path2, []byte(`{"services": {}}`), 0644); err != nil {
		t.Fatalf("failed to write json: %v", err)
	}
	cfg, err := LoadJSONConfigFromPaths([]string{path1, path2}, nil)
	if err != nil {
		t.Fatalf("LoadJSONConfigFromPaths failed: %v", err)
	}
	if cfg.Services.Plex != nil {
		t.Fatalf("expected zero services")
	}
}

func TestWrite(t *testing.T) {
	tempDir := t.TempDir()
	cfg := Config{}
	cfg.System.ComposeDir = "/opt/compose"
	cfg.System.TankMount = "/mnt/tank"

	dst := filepath.Join(tempDir, "config.json")
	if err := Write(dst, cfg); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	loaded, err := LoadJSONConfigFromPaths([]string{dst}, nil)
	if err != nil {
		t.Fatalf("failed to reload written config: %v", err)
	}
	if loaded.System.ComposeDir != "/opt/compose" {
		t.Fatalf("compose_dir mismatch: %q", loaded.System.ComposeDir)
	}
	if loaded.System.TankMount != "/mnt/tank" {
		t.Fatalf("tank_mount mismatch: %q", loaded.System.TankMount)
	}
}

func TestWrite_RoundTripPreservesServices(t *testing.T) {
	src := Config{}
	src.Services.Plex = []ServiceConfig{{
		Name: "Main", URL: "http://plex:32400", Token: "t1", Enabled: true,
	}}
	src.Services.Sonarr = []ServiceConfig{{
		Name: "HD", URL: "http://sonarr:8989", APIKey: "k1", Enabled: true,
	}}

	dst := filepath.Join(t.TempDir(), "config.json")
	if err := Write(dst, src); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	loaded, err := LoadJSONConfigFromPaths([]string{dst}, nil)
	if err != nil {
		t.Fatalf("failed to reload: %v", err)
	}
	if len(loaded.Services.Plex) != 1 || loaded.Services.Plex[0].Token != "t1" {
		t.Fatalf("plex config corrupted: %+v", loaded.Services.Plex)
	}
	if len(loaded.Services.Sonarr) != 1 || loaded.Services.Sonarr[0].APIKey != "k1" {
		t.Fatalf("sonarr config corrupted: %+v", loaded.Services.Sonarr)
	}
}

func TestGetConfigPaths_ReturnsExpectedOrder(t *testing.T) {
	paths := GetConfigPaths()
	if len(paths) < 1 {
		t.Fatal("expected at least one config path")
	}
	if !strings.HasSuffix(paths[0], filepath.Join(".config", "motd", "config.json")) {
		t.Fatalf("expected user config path, got %q", paths[0])
	}
}

func TestGetExplicitLegacyConfigPaths_ReturnsSiblingYAML(t *testing.T) {
	expectedDir := filepath.Join("some", "path")
	paths := GetExplicitLegacyConfigPaths(filepath.Join(string(filepath.Separator), expectedDir, "config.json"))
	if len(paths) != 2 {
		t.Fatalf("expected 2 legacy paths, got %d", len(paths))
	}
	if !strings.HasSuffix(paths[0], filepath.Join(string(filepath.Separator), expectedDir, "config.yml")) {
		t.Fatalf("expected config.yml sibling, got %q", paths[0])
	}
	if !strings.HasSuffix(paths[1], filepath.Join(string(filepath.Separator), expectedDir, "config.yaml")) {
		t.Fatalf("expected config.yaml sibling, got %q", paths[1])
	}
}

func TestAtomicWriteFile_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")

	if err := AtomicWriteFile(path, []byte("hello"), 0644); err != nil {
		t.Fatalf("AtomicWriteFile failed: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	if string(data) != "hello" {
		t.Fatalf("unexpected content: %q", data)
	}
}

func TestAtomicWriteFile_ReplacesExistingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")

	if err := os.WriteFile(path, []byte("old"), 0644); err != nil {
		t.Fatalf("failed to write initial file: %v", err)
	}

	if err := AtomicWriteFile(path, []byte("new"), 0644); err != nil {
		t.Fatalf("AtomicWriteFile failed: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	if string(data) != "new" {
		t.Fatalf("unexpected content: %q", data)
	}
}

func TestAtomicWriteFile_EmptyData(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.txt")

	if err := AtomicWriteFile(path, []byte{}, 0644); err != nil {
		t.Fatalf("AtomicWriteFile failed for empty data: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	if len(data) != 0 {
		t.Fatalf("expected empty file, got %d bytes", len(data))
	}
}

func TestAtomicWriteFile_SetsPermissions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "secure.txt")

	if err := AtomicWriteFile(path, []byte("secret"), 0600); err != nil {
		t.Fatalf("AtomicWriteFile failed: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("failed to stat file: %v", err)
	}
	if runtime.GOOS == "windows" {
		return
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("expected mode 0600, got %o", info.Mode().Perm())
	}
}

func TestWrite_AtomicReplacesExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	orig := Config{System: struct {
		ComposeDir string `json:"compose_dir"`
		TankMount  string `json:"tank_mount"`
		Network    struct {
			Interface string `json:"interface,omitempty"`
		} `json:"network,omitempty"`
	}{ComposeDir: "/orig", TankMount: "/mnt/orig"}}
	if err := Write(path, orig); err != nil {
		t.Fatalf("initial Write failed: %v", err)
	}

	updated := Config{System: struct {
		ComposeDir string `json:"compose_dir"`
		TankMount  string `json:"tank_mount"`
		Network    struct {
			Interface string `json:"interface,omitempty"`
		} `json:"network,omitempty"`
	}{ComposeDir: "/updated", TankMount: "/mnt/updated"}}
	if err := Write(path, updated); err != nil {
		t.Fatalf("second Write failed: %v", err)
	}

	loaded, err := LoadJSONConfigFromPaths([]string{path}, nil)
	if err != nil {
		t.Fatalf("failed to reload: %v", err)
	}
	if loaded.System.ComposeDir != "/updated" {
		t.Fatalf("expected /updated, got %q", loaded.System.ComposeDir)
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
