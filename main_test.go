package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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

func TestParseVnstatMonthlyUsage_InterfaceSelection(t *testing.T) {
	now := time.Date(2026, time.April, 10, 12, 0, 0, 0, time.UTC)
	payload := []byte(`{
		"interfaces":[
			{"id":"eth0","traffic":{"month":[{"rx":1073741824,"tx":1073741824,"date":{"year":2026,"month":4}}]}},
			{"id":"wlan0","traffic":{"month":[{"rx":2147483648,"tx":2147483648,"date":{"year":2026,"month":4}}]}}
		]
	}`)

	rxGB, txGB, _, _, err := parseVnstatMonthlyUsage(payload, "wlan0", now)
	if err != nil {
		t.Fatalf("parseVnstatMonthlyUsage failed: %v", err)
	}
	if rxGB != 2.0 || txGB != 2.0 {
		t.Fatalf("expected preferred wlan0 values, got rx=%.2f tx=%.2f", rxGB, txGB)
	}

	_, _, _, _, err = parseVnstatMonthlyUsage([]byte(`{"interfaces":[{"id":"eth0","traffic":{"month":[]}}]}`), "", now)
	if err == nil {
		t.Fatal("expected error when no monthly data is available")
	}
}

func TestCountUniqueWhoUsers(t *testing.T) {
	output := []byte("alice pts/0 2026-04-30 10:00\nbob pts/1 2026-04-30 10:01\nalice pts/2 2026-04-30 10:02\n")
	if got := countUniqueWhoUsers(output); got != 2 {
		t.Fatalf("expected 2 unique users, got %d", got)
	}
}

func TestCountNonEmptyLines(t *testing.T) {
	if got := countNonEmptyLines([]byte("one\n\n two \n")); got != 2 {
		t.Fatalf("expected 2 non-empty lines, got %d", got)
	}
}

func TestCountWindowsTasklistProcesses(t *testing.T) {
	output := []byte("System Idle Process              0 Services                   0          8 K\nSystem                           4 Services                   0        156 K\n\n")
	if got := countWindowsTasklistProcesses(output); got != 2 {
		t.Fatalf("expected 2 Windows tasklist rows, got %d", got)
	}
}

func TestParseWMICValueAndUint(t *testing.T) {
	output := []byte("Caption=Microsoft Windows 11 Pro\nTotalVisibleMemorySize=33554432\n")
	caption, ok := parseWMICValue(output, "Caption")
	if !ok || caption != "Microsoft Windows 11 Pro" {
		t.Fatalf("unexpected caption: %q ok=%v", caption, ok)
	}
	mem, ok := parseWMICUint(output, "TotalVisibleMemorySize")
	if !ok || mem != 33554432 {
		t.Fatalf("unexpected memory value: %d ok=%v", mem, ok)
	}
}

func TestParseWindowsOSPowerShell(t *testing.T) {
	info, ok := parseWindowsOSPowerShell([]byte("Microsoft Windows 11 IoT Enterprise LTSC|26100|26100.3915\n"))
	if !ok {
		t.Fatal("expected Windows PowerShell OS output to parse")
	}
	if info.Version != "11" || info.Edition != "IoT Enterprise LTSC" || info.Build != "26100.3915" {
		t.Fatalf("unexpected Windows OS info: %+v", info)
	}
}

func TestParseWindowsOSWMIC(t *testing.T) {
	info, ok := parseWindowsOSWMIC([]byte("Caption=Microsoft Windows 10 Enterprise\nBuildNumber=19045\n"))
	if !ok {
		t.Fatal("expected Windows WMIC OS output to parse")
	}
	if info.Version != "10" || info.Edition != "Enterprise" || info.Build != "19045" {
		t.Fatalf("unexpected Windows OS info: %+v", info)
	}
}

func TestInferWindowsVersionUsesBuildNumber(t *testing.T) {
	if got := inferWindowsVersion("Microsoft Windows IoT Enterprise", "26100"); got != "11" {
		t.Fatalf("expected build 26100 to infer Windows 11, got %q", got)
	}
	if got := inferWindowsVersion("Microsoft Windows Enterprise", "19045"); got != "10" {
		t.Fatalf("expected build 19045 to infer Windows 10, got %q", got)
	}
}

func TestParseWindowsEdition(t *testing.T) {
	if got := parseWindowsEdition("Microsoft Windows 11 IoT Enterprise LTSC"); got != "IoT Enterprise LTSC" {
		t.Fatalf("unexpected edition: %q", got)
	}
}

func TestParseWMICDateTime(t *testing.T) {
	parsed, ok := parseWMICDateTime("20260430123456.000000+000")
	if !ok {
		t.Fatal("expected WMIC datetime to parse")
	}
	if parsed.Year() != 2026 || parsed.Month() != time.April || parsed.Day() != 30 || parsed.Hour() != 12 {
		t.Fatalf("unexpected datetime: %s", parsed)
	}
}

func TestFormatDuration(t *testing.T) {
	formatted := formatDuration((49 * time.Hour) + (5 * time.Minute))
	if formatted != "2 days, 1 hour, 5 minutes" {
		t.Fatalf("unexpected duration: %s", formatted)
	}
}

func TestFormatDurationNegative(t *testing.T) {
	if got := formatDuration(-time.Minute); got != "0 minutes" {
		t.Fatalf("expected negative duration to clamp to zero, got %q", got)
	}
}

func TestParseWindowsCPUPercent(t *testing.T) {
	percent, ok := parseWindowsCPUPercent([]byte("LoadPercentage=10\nLoadPercentage=30\n"))
	if !ok || percent != 20 {
		t.Fatalf("unexpected CPU percent: %d ok=%v", percent, ok)
	}
}

func TestParseWindowsMemoryKB(t *testing.T) {
	total, free, ok := parseWindowsMemoryKB([]byte("33554432,16777216"))
	if !ok || total != 33554432 || free != 16777216 {
		t.Fatalf("unexpected memory parse total=%d free=%d ok=%v", total, free, ok)
	}
}

func TestParseWindowsMemoryKBRejectsMalformedInput(t *testing.T) {
	if _, _, ok := parseWindowsMemoryKB([]byte("bad")); ok {
		t.Fatal("expected malformed memory output to fail")
	}
	if _, _, ok := parseWindowsMemoryKB([]byte("1,not-a-number")); ok {
		t.Fatal("expected non-numeric memory output to fail")
	}
}

func TestParseWindowsDiskCSV(t *testing.T) {
	disks := parseWindowsDiskCSV([]byte("bad\nC:,1000,250\nD:,2000,1000\nE:,not-a-number,10\n"))
	if len(disks) != 2 {
		t.Fatalf("expected 2 disks, got %d", len(disks))
	}
	if disks[0].Name != "C:" || disks[0].SizeBytes != 1000 || disks[0].FreeBytes != 250 {
		t.Fatalf("unexpected first disk: %+v", disks[0])
	}
}

func TestParseWindowsDiskWMIC(t *testing.T) {
	disks := parseWindowsDiskWMIC([]byte("DeviceID=C:\nFreeSpace=250\nSize=1000\n\nDeviceID=D:\nFreeSpace=1000\nSize=2000\n"))
	if len(disks) != 2 {
		t.Fatalf("expected 2 disks, got %d", len(disks))
	}
	if disks[1].Name != "D:" || disks[1].SizeBytes != 2000 || disks[1].FreeBytes != 1000 {
		t.Fatalf("unexpected second disk: %+v", disks[1])
	}
}

func TestParseWindowsTemperature(t *testing.T) {
	celsius, ok := parseWindowsTemperature([]byte("3032"))
	if !ok {
		t.Fatal("expected temperature to parse")
	}
	if celsius < 30.0 || celsius > 30.1 {
		t.Fatalf("unexpected celsius value: %.2f", celsius)
	}
}

func TestParseWindowsTemperatureRejectsMalformedInput(t *testing.T) {
	for _, input := range [][]byte{[]byte(""), []byte("not-a-number"), []byte("0")} {
		if _, ok := parseWindowsTemperature(input); ok {
			t.Fatalf("expected temperature input %q to fail", input)
		}
	}
}

func TestServiceURLAndLabel(t *testing.T) {
	if got := serviceURL("http://host/", "/api"); got != "http://host/api" {
		t.Fatalf("unexpected service URL: %q", got)
	}
	if got := serviceURL("http://host", "/api"); got != "http://host/api" {
		t.Fatalf("unexpected service URL without trailing slash: %q", got)
	}
	if got := serviceLabel("Plex", ""); got != "Plex" {
		t.Fatalf("unexpected empty service label: %q", got)
	}
	if got := serviceLabel("Plex", "Default"); got != "Plex" {
		t.Fatalf("unexpected default service label: %q", got)
	}
	if got := serviceLabel("Plex", "Main"); got != "Plex (Main)" {
		t.Fatalf("unexpected named service label: %q", got)
	}
}

func TestRenderJellyfinInstance_RequestAndOutput(t *testing.T) {
	originalClient := httpClient
	t.Cleanup(func() { httpClient = originalClient })

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/Sessions" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if r.Header.Get("X-Emby-Token") != "jellyfin-token" || !strings.Contains(r.Header.Get("Authorization"), "jellyfin-token") {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `[{"NowPlayingItem":{"Id":"1"},"PlayState":{"PlayMethod":"Transcode"},"TranscodingInfo":{"Bitrate":5000000}}]`)
	}))
	defer server.Close()
	httpClient = server.Client()

	line, ok := renderJellyfinInstance(ServiceConfig{Name: "Main", URL: server.URL, Token: "jellyfin-token", Enabled: true})
	if !ok {
		t.Fatal("expected Jellyfin output")
	}
	if !strings.Contains(line, "Jellyfin (Main)") || !strings.Contains(line, "1 streams, 1 transcode") || !strings.Contains(line, "5.00 Mbps") {
		t.Fatalf("unexpected Jellyfin output: %q", line)
	}
}

func TestRenderRadarrInstance_RequestAndPluralization(t *testing.T) {
	originalClient := httpClient
	t.Cleanup(func() { httpClient = originalClient })

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3/wanted/missing" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if r.Header.Get("X-Api-Key") != "radarr-key" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"totalRecords":1,"records":[]}`)
	}))
	defer server.Close()
	httpClient = server.Client()

	line, ok := renderRadarrInstance(ServiceConfig{Name: "HD", URL: server.URL, APIKey: "radarr-key", Enabled: true})
	if !ok {
		t.Fatal("expected Radarr output")
	}
	if !strings.Contains(line, "Radarr (HD)") || !strings.Contains(line, "1 missing movie") || strings.Contains(line, "movies") {
		t.Fatalf("unexpected Radarr output: %q", line)
	}
}

func TestRenderPlexInstance_ActiveTranscodes(t *testing.T) {
	originalClient := httpClient
	t.Cleanup(func() { httpClient = originalClient })

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/status/sessions" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if r.Header.Get("X-Plex-Token") != "plex-token" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/xml")
		_, _ = fmt.Fprint(w, `<MediaContainer size="2"><Video><TranscodeSession videoDecision="transcode"></TranscodeSession><Session bandwidth="4000"></Session></Video><Video><Session bandwidth="2000"></Session></Video></MediaContainer>`)
	}))
	defer server.Close()
	httpClient = server.Client()

	line, ok := renderPlexInstance(ServiceConfig{Name: "Main", URL: server.URL, Token: "plex-token", Enabled: true})
	if !ok {
		t.Fatal("expected Plex output")
	}
	if !strings.Contains(line, "Plex (Main)") || !strings.Contains(line, "2 streams, 1 transcode") || !strings.Contains(line, "6.00 Mbps") {
		t.Fatalf("unexpected Plex output: %q", line)
	}
}

func TestRenderMediaLine(t *testing.T) {
	line := formatMediaLine("Sonarr", "No missing episodes", GREEN)
	if !strings.Contains(line, "Sonarr") || !strings.Contains(line, "No missing episodes") {
		t.Fatalf("unexpected media line: %q", line)
	}
}

func TestShowMediaServicesStableOrder(t *testing.T) {
	originalConfig := config
	originalClient := httpClient
	t.Cleanup(func() {
		config = originalConfig
		httpClient = originalClient
	})

	plexServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		_, _ = fmt.Fprint(w, `<MediaContainer size="0"></MediaContainer>`)
	}))
	defer plexServer.Close()

	sonarrServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"totalRecords":2,"records":[]}`)
	}))
	defer sonarrServer.Close()

	config = Config{}
	config.Services.Plex = []ServiceConfig{{Name: "Main", URL: plexServer.URL, Token: "plex-token", Enabled: true}}
	config.Services.Sonarr = []ServiceConfig{{Name: "HD", URL: sonarrServer.URL, APIKey: "sonarr-token", Enabled: true}}
	httpClient = plexServer.Client()

	output := captureStdout(showMediaServices)
	plexIndex := strings.Index(output, "Plex")
	sonarrIndex := strings.Index(output, "Sonarr")
	if plexIndex < 0 || sonarrIndex < 0 {
		t.Fatalf("expected Plex and Sonarr output, got: %q", output)
	}
	if plexIndex > sonarrIndex {
		t.Fatalf("expected stable service order, got: %q", output)
	}
}

func TestShowMediaServicesSkipsFailedServices(t *testing.T) {
	originalConfig := config
	originalClient := httpClient
	t.Cleanup(func() {
		config = originalConfig
		httpClient = originalClient
	})

	seerrServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"pending":3}`)
	}))
	defer seerrServer.Close()

	config = Config{}
	config.Services.Sonarr = []ServiceConfig{{Name: "Bad", URL: "http://127.0.0.1:1", APIKey: "sonarr-token", Enabled: true}}
	config.Services.Seerr = []ServiceConfig{{Name: "Main", URL: seerrServer.URL, APIKey: "seerr-token", Enabled: true}}
	httpClient = seerrServer.Client()

	output := captureStdout(showMediaServices)
	if strings.Contains(output, "Sonarr") {
		t.Fatalf("expected failed Sonarr to be skipped, got: %q", output)
	}
	if !strings.Contains(output, "Seerr") || !strings.Contains(output, "3 pending requests") {
		t.Fatalf("expected successful Seerr output, got: %q", output)
	}
}

func captureStdout(fn func()) string {
	reader, writer, err := os.Pipe()
	if err != nil {
		panic(err)
	}

	stdout := os.Stdout
	os.Stdout = writer
	defer func() { os.Stdout = stdout }()

	fn()
	_ = writer.Close()

	var buffer bytes.Buffer
	_, _ = io.Copy(&buffer, reader)
	_ = reader.Close()
	return buffer.String()
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

func TestPlatformAssetName(t *testing.T) {
	tests := []struct {
		goos   string
		goarch string
		want   string
	}{
		{goos: "linux", goarch: "amd64", want: "motd-linux-amd64"},
		{goos: "linux", goarch: "arm64", want: "motd-linux-arm64"},
		{goos: "darwin", goarch: "amd64", want: "motd-darwin-amd64"},
		{goos: "darwin", goarch: "arm64", want: "motd-darwin-arm64"},
		{goos: "windows", goarch: "amd64", want: "motd-windows-amd64.exe"},
		{goos: "windows", goarch: "arm64", want: ""},
	}

	for _, tt := range tests {
		if got := platformAssetName(tt.goos, tt.goarch); got != tt.want {
			t.Fatalf("platformAssetName(%s,%s)=%q want %q", tt.goos, tt.goarch, got, tt.want)
		}
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
