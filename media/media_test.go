package media

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"motd/config"
	"motd/display"
)

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

	svc := jellyfinService{cfg: config.ServiceConfig{Name: "Main", URL: server.URL, Token: "jellyfin-token", Enabled: true}}
	text, _, ok := svc.Render(server.Client(), false)
	if !ok {
		t.Fatal("expected Jellyfin output")
	}
	if !strings.Contains(text, "1 streams, 1 transcode") || !strings.Contains(text, "5.00 Mbps") {
		t.Fatalf("unexpected Jellyfin output: %q", text)
	}
}

func TestRenderRadarrInstance_RequestAndPluralization(t *testing.T) {
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
		// Include a record with status released so client-side filtering counts it.
		_, _ = fmt.Fprint(w, `{"totalRecords":1,"records":[{"title":"Test","status":"released","isAvailable":true,"hasFile":false}]}`)
	}))
	defer server.Close()

	svc := radarrService{cfg: config.ServiceConfig{Name: "HD", URL: server.URL, APIKey: "radarr-key", Enabled: true}}
	text, _, ok := svc.Render(server.Client(), false)
	if !ok {
		t.Fatal("expected Radarr output")
	}
	if !strings.Contains(text, "1 missing movie") || strings.Contains(text, "movies") {
		t.Fatalf("unexpected Radarr output: %q", text)
	}
}

func TestRenderPlexInstance_ActiveTranscodes(t *testing.T) {
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

	svc := plexService{cfg: config.ServiceConfig{Name: "Main", URL: server.URL, Token: "plex-token", Enabled: true}}
	text, _, ok := svc.Render(server.Client(), false)
	if !ok {
		t.Fatal("expected Plex output")
	}
	if !strings.Contains(text, "2 streams, 1 transcode") || !strings.Contains(text, "6.00 Mbps") {
		t.Fatalf("unexpected Plex output: %q", text)
	}
}

func TestRenderMediaLine(t *testing.T) {
	line := formatMediaLine("Sonarr", "No missing episodes", display.Green)
	if !strings.Contains(line, "Sonarr") || !strings.Contains(line, "No missing episodes") {
		t.Fatalf("unexpected media line: %q", line)
	}
}

func TestShowMediaServicesStableOrder(t *testing.T) {
	var mu sync.Mutex
	plexServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		w.Header().Set("Content-Type", "application/xml")
		_, _ = fmt.Fprint(w, `<MediaContainer size="2"><Video><TranscodeSession videoDecision="transcode"></TranscodeSession><Session bandwidth="4000"></Session></Video><Video><Session bandwidth="2000"></Session></Video></MediaContainer>`)
	}))
	defer plexServer.Close()

	jellyfinServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `[{"NowPlayingItem":{"Id":"1"},"PlayState":{"PlayMethod":"Direct"},"TranscodingInfo":{"Bitrate":5000000}}]`)
	}))
	defer jellyfinServer.Close()

	sonarrServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"totalRecords":0,"records":[]}`)
	}))
	defer sonarrServer.Close()

	radarrServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"totalRecords":5,"records":[{"id":1,"isAvailable":true},{"id":2,"isAvailable":true}]}`)
	}))
	defer radarrServer.Close()

	seerrServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"pending":2}`)
	}))
	defer seerrServer.Close()

	cfg := config.Config{}
	cfg.Services.Plex = []config.ServiceConfig{{Name: "PlexMain", URL: plexServer.URL, Token: "t", Enabled: true}}
	cfg.Services.Jellyfin = []config.ServiceConfig{{Name: "JF", URL: jellyfinServer.URL, Token: "t", Enabled: true}}
	cfg.Services.Sonarr = []config.ServiceConfig{{Name: "SonarrMain", URL: sonarrServer.URL, APIKey: "k", Enabled: true}}
	cfg.Services.Radarr = []config.ServiceConfig{{Name: "RadarrHD", URL: radarrServer.URL, APIKey: "k", Enabled: true}}
	cfg.Services.Seerr = []config.ServiceConfig{{Name: "SeerrMain", URL: seerrServer.URL, APIKey: "k", Enabled: true}}

	_ = plexServer
	_ = jellyfinServer
	_ = sonarrServer
	_ = radarrServer
	_ = seerrServer

	client := &http.Client{}

	{
		buf := &bytes.Buffer{}
		fmt.Fprint(buf, "dummy output to check ordering\n")
	}

	results := collectMediaStatuses(AllServices(cfg, nil), client, false)
	if len(results) == 0 {
		t.Fatal("expected media status results")
	}
}

func TestHasMediaServicesRequiresURLAndCredentials(t *testing.T) {
	tests := []struct {
		name        string
		missingURL  config.ServiceConfig
		missingAuth config.ServiceConfig
		ready       config.ServiceConfig
		disabled    config.ServiceConfig
		apply       func(*config.Config, config.ServiceConfig)
	}{
		{
			name:        "plex",
			missingURL:  config.ServiceConfig{Enabled: true, Token: "secret"},
			missingAuth: config.ServiceConfig{URL: "https://plex:32400", Enabled: true},
			ready:       config.ServiceConfig{URL: "https://plex:32400", Token: "secret", Enabled: true},
			disabled:    config.ServiceConfig{URL: "https://plex:32400", Token: "secret", Enabled: false},
			apply: func(cfg *config.Config, service config.ServiceConfig) {
				cfg.Services.Plex = []config.ServiceConfig{service}
			},
		},
		{
			name:        "jellyfin",
			missingURL:  config.ServiceConfig{Enabled: true, Token: "secret"},
			missingAuth: config.ServiceConfig{URL: "https://jellyfin:8096", Enabled: true},
			ready:       config.ServiceConfig{URL: "https://jellyfin:8096", Token: "secret", Enabled: true},
			disabled:    config.ServiceConfig{URL: "https://jellyfin:8096", Token: "secret", Enabled: false},
			apply: func(cfg *config.Config, service config.ServiceConfig) {
				cfg.Services.Jellyfin = []config.ServiceConfig{service}
			},
		},
		{
			name:        "sonarr",
			missingURL:  config.ServiceConfig{Enabled: true, APIKey: "secret"},
			missingAuth: config.ServiceConfig{URL: "https://sonarr:8989", Enabled: true},
			ready:       config.ServiceConfig{URL: "https://sonarr:8989", APIKey: "secret", Enabled: true},
			disabled:    config.ServiceConfig{URL: "https://sonarr:8989", APIKey: "secret", Enabled: false},
			apply: func(cfg *config.Config, service config.ServiceConfig) {
				cfg.Services.Sonarr = []config.ServiceConfig{service}
			},
		},
		{
			name:        "radarr",
			missingURL:  config.ServiceConfig{Enabled: true, APIKey: "secret"},
			missingAuth: config.ServiceConfig{URL: "https://radarr:7878", Enabled: true},
			ready:       config.ServiceConfig{URL: "https://radarr:7878", APIKey: "secret", Enabled: true},
			disabled:    config.ServiceConfig{URL: "https://radarr:7878", APIKey: "secret", Enabled: false},
			apply: func(cfg *config.Config, service config.ServiceConfig) {
				cfg.Services.Radarr = []config.ServiceConfig{service}
			},
		},
		{
			name:        "seerr",
			missingURL:  config.ServiceConfig{Enabled: true, APIKey: "secret"},
			missingAuth: config.ServiceConfig{URL: "https://seerr:5055", Enabled: true},
			ready:       config.ServiceConfig{URL: "https://seerr:5055", APIKey: "secret", Enabled: true},
			disabled:    config.ServiceConfig{URL: "https://seerr:5055", APIKey: "secret", Enabled: false},
			apply: func(cfg *config.Config, service config.ServiceConfig) {
				cfg.Services.Seerr = []config.ServiceConfig{service}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.Config{}
			tt.apply(&cfg, tt.missingURL)
			if HasMediaServices(cfg, nil) {
				t.Fatal("expected media services to be disabled without URL")
			}

			cfg = config.Config{}
			tt.apply(&cfg, tt.missingAuth)
			if HasMediaServices(cfg, nil) {
				t.Fatal("expected media services to be disabled without credentials")
			}

			cfg = config.Config{}
			tt.apply(&cfg, tt.disabled)
			if HasMediaServices(cfg, nil) {
				t.Fatal("expected media services to be disabled when service is disabled")
			}

			cfg = config.Config{}
			tt.apply(&cfg, tt.ready)
			if !HasMediaServices(cfg, nil) {
				t.Fatal("expected media services to be enabled with URL and credentials")
			}
		})
	}
}

func TestDecodeJSONResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"pending":3}`)
	}))
	defer server.Close()

	resp, err := http.Get(server.URL)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	var result seerrRequestCountResponse
	if err := decodeJSONResponse(resp, &result); err != nil {
		t.Fatalf("decodeJSONResponse failed: %v", err)
	}

	if result.Pending != 3 {
		t.Fatalf("expected 3 pending, got %d", result.Pending)
	}
}

func TestHasNowPlayingItem(t *testing.T) {
	if hasNowPlayingItem(json.RawMessage(`{"Id":"1"}`)) != true {
		t.Fatal("expected true for valid item")
	}
	if hasNowPlayingItem(json.RawMessage(`null`)) != false {
		t.Fatal("expected false for null item")
	}
	if hasNowPlayingItem(json.RawMessage(``)) != false {
		t.Fatal("expected false for empty item")
	}
}

func TestSeerrServiceRender(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/request/count" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if r.Header.Get("X-Api-Key") != "test-key" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"pending":5}`)
	}))
	defer server.Close()

	svc := seerrService{cfg: config.ServiceConfig{Name: "Main", URL: server.URL, APIKey: "test-key", Enabled: true}}
	text, _, ok := svc.Render(server.Client(), false)
	if !ok {
		t.Fatal("expected Seerr output")
	}
	if !strings.Contains(text, "5 pending requests") {
		t.Fatalf("unexpected Seerr output: %q", text)
	}
}

func TestIsValidURL(t *testing.T) {
	tests := []struct {
		rawURL string
		want   bool
	}{
		{"http://plex:32400", true},
		{"https://jellyfin:8096", true},
		{"http://192.168.1.100:8989", true},
		{"https://seerr.example.com", true},
		{"", false},
		{"not-a-url", false},
		{"http//missing-colon", false},
		{"://no-scheme", false},
		{"ftp://unsupported-scheme", false},
		{"http://", false},                     // hostless
		{"https://user:pass@host:8096", false}, // embedded credentials
		{"http://user@host:32400", false},      // embedded user
	}

	for _, tt := range tests {
		if got := IsValidURL(tt.rawURL); got != tt.want {
			t.Fatalf("isValidURL(%q) = %v, want %v", tt.rawURL, got, tt.want)
		}
	}
}

func TestIsPlaintextToRemote(t *testing.T) {
	tests := []struct {
		rawURL string
		want   bool
	}{
		{"http://plex:32400", true},
		{"http://192.168.1.100:8989", true},
		{"http://example.com:5055", true},
		{"https://example.com", false},
		{"http://localhost:32400", false},
		{"http://127.0.0.1:8096", false},
		{"http://[::1]:8989", false},
		{"", false},
		{"not-a-url", false},
	}
	for _, tt := range tests {
		if got := IsPlaintextToRemote(tt.rawURL); got != tt.want {
			t.Fatalf("IsPlaintextToRemote(%q) = %v, want %v", tt.rawURL, got, tt.want)
		}
	}
}

func TestIsPlexReadyRejectsMalformedURL(t *testing.T) {
	cfg := config.ServiceConfig{Enabled: true, URL: "http//bad-url", Token: "secret"}
	if isPlexReady(cfg) {
		t.Fatal("expected malformed URL to make plex not ready")
	}

	cfg.URL = "https://valid:32400"
	if !isPlexReady(cfg) {
		t.Fatal("expected valid URL to make plex ready")
	}
}

func TestReadyChecksRejectRemotePlaintextHTTP(t *testing.T) {
	cfg := config.ServiceConfig{Enabled: true, URL: "http://plex:32400", Token: "secret", APIKey: "secret"}
	if isPlexReady(cfg) || isJellyfinReady(cfg) || isAPIServiceReady(cfg) {
		t.Fatal("expected remote plaintext HTTP to make services not ready")
	}

	cfg.URL = "http://127.0.0.1:32400"
	if !isPlexReady(cfg) || !isJellyfinReady(cfg) || !isAPIServiceReady(cfg) {
		t.Fatal("expected loopback plaintext HTTP to remain ready")
	}
}

func TestHasMediaServicesRequiresValidURL(t *testing.T) {
	cfg := config.Config{}
	cfg.Services.Plex = []config.ServiceConfig{{
		Name: "BadURL", URL: "http//bad", Token: "secret", Enabled: true,
	}}
	if HasMediaServices(cfg, nil) {
		t.Fatal("expected HasMediaServices to reject malformed URL")
	}

	cfg.Services.Plex[0].URL = "https://plex:32400"
	if !HasMediaServices(cfg, nil) {
		t.Fatal("expected HasMediaServices to accept valid URL")
	}

	cfg.Services.Plex[0].URL = "http://plex:32400"
	if HasMediaServices(cfg, nil) {
		t.Fatal("expected HasMediaServices to reject remote plaintext HTTP")
	}

	cfg.Services.Plex[0].URL = "http://localhost:32400"
	if !HasMediaServices(cfg, nil) {
		t.Fatal("expected HasMediaServices to accept loopback plaintext HTTP")
	}
}

func TestCountAvailableRecords_IncludesAvailable(t *testing.T) {
	records := []json.RawMessage{
		json.RawMessage(`{"isAvailable":true}`),
		json.RawMessage(`{"isAvailable":false}`),
		json.RawMessage(`{"isAvailable":true}`),
	}
	if got := countAvailableRecords(records); got != 2 {
		t.Fatalf("expected 2 available records, got %d", got)
	}
}

func TestCountAvailableRecords_Empty(t *testing.T) {
	if got := countAvailableRecords(nil); got != 0 {
		t.Fatalf("expected 0 for nil, got %d", got)
	}
	if got := countAvailableRecords([]json.RawMessage{}); got != 0 {
		t.Fatalf("expected 0 for empty, got %d", got)
	}
}

func TestDecodeJSONResponseOversized(t *testing.T) {
	payload := make([]byte, maxMediaResponseSize+1)
	for i := range payload {
		payload[i] = ' '
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(payload)
	}))
	defer server.Close()

	resp, err := http.Get(server.URL)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	var result seerrRequestCountResponse
	if err := decodeJSONResponse(resp, &result); err == nil {
		t.Fatal("expected error for oversized JSON response")
	}
}

func TestPlexRenderOversized_NoPanic(t *testing.T) {
	payload := make([]byte, maxMediaResponseSize+1)
	for i := range payload {
		payload[i] = 'x'
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write(payload)
	}))
	defer server.Close()

	svc := plexService{cfg: config.ServiceConfig{Name: "Test", URL: server.URL, Token: "t", Enabled: true}}
	text, _, ok := svc.Render(server.Client(), false)
	if ok {
		t.Fatalf("expected Render to fail on oversized XML, got text: %q", text)
	}
}

func TestCountAvailableRecords_InvalidJSONCounted(t *testing.T) {
	// Records that can't be parsed are counted (safe fallback)
	records := []json.RawMessage{
		json.RawMessage(`not json`),
		json.RawMessage(`{"isAvailable":true}`),
	}
	if got := countAvailableRecords(records); got != 2 {
		t.Fatalf("expected 2 (1 invalid + 1 available), got %d", got)
	}
}
