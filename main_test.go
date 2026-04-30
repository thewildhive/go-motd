package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

func TestParseARRMissingCount(t *testing.T) {
	withTotal := arrWantedMissingResponse{TotalRecords: 42, Records: []json.RawMessage{json.RawMessage(`{}`)}}
	if got := parseARRMissingCount(withTotal); got != 42 {
		t.Fatalf("expected totalRecords to win, got %d", got)
	}

	withoutTotal := arrWantedMissingResponse{Records: []json.RawMessage{json.RawMessage(`{}`), json.RawMessage(`{}`)}}
	if got := parseARRMissingCount(withoutTotal); got != 2 {
		t.Fatalf("expected len(records), got %d", got)
	}
}

func TestParseJellyfinSessions(t *testing.T) {
	sessions := []jellyfinSession{
		{
			NowPlayingItem: json.RawMessage(`{"Id":"a"}`),
			TranscodingInfo: &struct {
				Bitrate int64 `json:"Bitrate"`
			}{Bitrate: 4_000_000},
		},
		{
			NowPlayingItem: json.RawMessage(`{"Id":"b"}`),
			PlayState: struct {
				PlayMethod string `json:"PlayMethod"`
			}{PlayMethod: "Transcode"},
			TranscodingInfo: &struct {
				Bitrate int64 `json:"Bitrate"`
			}{Bitrate: 6_000_000},
		},
		{NowPlayingItem: json.RawMessage(`null`)},
	}

	active, transcodes, mbps, hasBW := parseJellyfinSessions(sessions)
	if active != 2 {
		t.Fatalf("expected 2 active streams, got %d", active)
	}
	if transcodes != 1 {
		t.Fatalf("expected 1 transcode, got %d", transcodes)
	}
	if !hasBW {
		t.Fatal("expected hasBW=true")
	}
	if mbps != 10.0 {
		t.Fatalf("expected 10.0 Mbps, got %.2f", mbps)
	}
}

func TestPickLatestVnstatMonth(t *testing.T) {
	now := time.Date(2026, time.April, 15, 10, 0, 0, 0, time.UTC)
	entries := []vnstatMonthlyEntry{
		{Date: struct {
			Year  int `json:"year"`
			Month int `json:"month"`
		}{Year: 2026, Month: 3}},
		{Date: struct {
			Year  int `json:"year"`
			Month int `json:"month"`
		}{Year: 2026, Month: 4}},
	}

	picked, ok := pickLatestVnstatMonth(entries, now)
	if !ok {
		t.Fatal("expected month selection")
	}
	if picked.Date.Month != 4 {
		t.Fatalf("expected current month, got %d", picked.Date.Month)
	}
}

func TestParseVnstatMonthlyUsage(t *testing.T) {
	now := time.Date(2026, time.April, 10, 12, 0, 0, 0, time.UTC)
	payload := []byte(`{
		"interfaces":[
			{"id":"eth0","traffic":{"month":[{"rx":1073741824,"tx":2147483648,"date":{"year":2026,"month":4}}]}}
		]
	}`)

	rxGB, txGB, rxEst, txEst, err := parseVnstatMonthlyUsage(payload, "eth0", now)
	if err != nil {
		t.Fatalf("parseVnstatMonthlyUsage failed: %v", err)
	}

	if rxGB != 1.0 || txGB != 2.0 {
		t.Fatalf("unexpected usage values rx=%.2f tx=%.2f", rxGB, txGB)
	}

	expectedRxEst := 1.0 * (30.0 / 10.0)
	expectedTxEst := 2.0 * (30.0 / 10.0)
	if rxEst != expectedRxEst || txEst != expectedTxEst {
		t.Fatalf("unexpected estimates rx=%.2f tx=%.2f", rxEst, txEst)
	}
}

func TestPluralSuffix(t *testing.T) {
	if pluralSuffix(1) != "" {
		t.Fatal("expected no suffix for singular")
	}
	if pluralSuffix(2) != "s" {
		t.Fatal("expected plural suffix for count>1")
	}
}

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		current string
		latest  string
		expect  int
	}{
		{current: "1.0.0", latest: "1.0.1", expect: -1},
		{current: "1.2.0", latest: "1.2.0", expect: 0},
		{current: "2.0.0", latest: "1.9.9", expect: 1},
	}

	for _, tt := range tests {
		if got := compareVersions(tt.current, tt.latest); got != tt.expect {
			t.Fatalf("compareVersions(%s,%s)=%d want %d", tt.current, tt.latest, got, tt.expect)
		}
	}
}

func TestGetPlatformAssetName_NonEmptyOnCurrentPlatform(t *testing.T) {
	if got := getPlatformAssetName(); got == "" {
		t.Fatal("expected non-empty asset name for current platform")
	}
}

func TestFetchSeerrPendingCount(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/request/count" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}

		if got := r.Header.Get("X-Api-Key"); got != "secret" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"pending":7}`)
	}))
	defer server.Close()

	pending, err := fetchSeerrPendingCount(server.Client(), server.URL, "secret")
	if err != nil {
		t.Fatalf("fetchSeerrPendingCount failed: %v", err)
	}

	if pending != 7 {
		t.Fatalf("expected pending=7, got %d", pending)
	}
}

func TestFetchSeerrPendingCount_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	}))
	defer server.Close()

	_, err := fetchSeerrPendingCount(server.Client(), server.URL, "bad-key")
	if err == nil {
		t.Fatal("expected error for non-200 response")
	}
}
