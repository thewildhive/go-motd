package main

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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

func TestParseChecksumLine(t *testing.T) {
	checksum, filename, ok := parseChecksumLine("abc123  *motd-linux-amd64")
	if !ok {
		t.Fatal("expected checksum line to parse")
	}
	if checksum != "abc123" || filename != "motd-linux-amd64" {
		t.Fatalf("unexpected checksum parse result: %q %q", checksum, filename)
	}
}

func TestDownloadBinaryUsesRequestedTempDirAndChecksum(t *testing.T) {
	payload := []byte("new binary")
	checksum := fmt.Sprintf("%x", sha256.Sum256(payload))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(payload)
	}))
	defer server.Close()

	originalClient := httpClient
	t.Cleanup(func() { httpClient = originalClient })
	httpClient = server.Client()

	tempDir := t.TempDir()
	path, err := downloadBinary(server.URL, "motd-linux-amd64", map[string]string{"motd-linux-amd64": checksum}, tempDir)
	if err != nil {
		t.Fatalf("downloadBinary failed: %v", err)
	}
	defer os.Remove(path)

	if filepath.Dir(path) != tempDir {
		t.Fatalf("expected temp file in %s, got %s", tempDir, path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read temp binary: %v", err)
	}
	if string(data) != string(payload) {
		t.Fatalf("unexpected downloaded payload: %q", data)
	}
}

func TestDownloadBinaryRejectsChecksumMismatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("new binary"))
	}))
	defer server.Close()

	originalClient := httpClient
	t.Cleanup(func() { httpClient = originalClient })
	httpClient = server.Client()

	_, err := downloadBinary(server.URL, "motd-linux-amd64", map[string]string{"motd-linux-amd64": "bad"}, t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "checksum verification failed") {
		t.Fatalf("expected checksum mismatch error, got %v", err)
	}
}

func TestWindowsBatchPath(t *testing.T) {
	if got := windowsBatchPath(`C:/Temp/motd-update.tmp`); got != `C:\Temp\motd-update.tmp` {
		t.Fatalf("unexpected Windows batch path: %q", got)
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
