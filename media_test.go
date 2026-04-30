package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
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
