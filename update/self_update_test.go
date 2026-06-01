package update

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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
		if got := CompareVersions(tt.current, tt.latest); got != tt.expect {
			t.Fatalf("CompareVersions(%s,%s)=%d want %d", tt.current, tt.latest, got, tt.expect)
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

	client := server.Client()
	tempDir := t.TempDir()
	path, err := downloadBinary(server.URL, "motd-linux-amd64", map[string]string{"motd-linux-amd64": checksum}, tempDir, client)
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

	client := server.Client()
	_, err := downloadBinary(server.URL, "motd-linux-amd64", map[string]string{"motd-linux-amd64": "bad"}, t.TempDir(), client)
	if err == nil || !strings.Contains(err.Error(), "checksum verification failed") {
		t.Fatalf("expected checksum mismatch error, got %v", err)
	}
}

func TestWindowsBatchPath(t *testing.T) {
	if got := windowsBatchPath(`C:/Temp/motd-update.tmp`); got != `C:\Temp\motd-update.tmp` {
		t.Fatalf("unexpected Windows batch path: %q", got)
	}
}

func TestWindowsBatchPathQuoteEscaping(t *testing.T) {
	if got := windowsBatchPath(`C:/"Program"/motd`); got != `C:\""Program""\motd` {
		t.Fatalf("unexpected quoted Windows batch path: %q", got)
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

func TestCheckWriteAccess_WritableDir(t *testing.T) {
	tempDir := t.TempDir()
	execPath := filepath.Join(tempDir, "motd")
	if err := checkWriteAccess(execPath); err != nil {
		t.Fatalf("expected write access to temp dir, got: %v", err)
	}
}

func TestCacheDir_CreatesDirectory(t *testing.T) {
	dir, err := cacheDir()
	if err != nil {
		t.Fatalf("cacheDir failed: %v", err)
	}
	defer os.RemoveAll(filepath.Dir(dir))
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("cache dir does not exist after cacheDir(): %v", err)
	}
}

func TestWriteAndReadCachedVersion(t *testing.T) {
	orig := cachePath
	t.Cleanup(func() { cachePath = orig })

	cacheDir := t.TempDir()
	cachePath = func() string { return filepath.Join(cacheDir, "cache") }

	// Write a message
	writeCachedVersion("update available")

	// Read it back (within cache interval)
	msg := readCachedVersion()
	if msg != "update available" {
		t.Fatalf("expected cached message, got %q", msg)
	}
}

func TestCachedVersionExpires(t *testing.T) {
	orig := cachePath
	t.Cleanup(func() { cachePath = orig })

	cacheDir := t.TempDir()
	cachePath = func() string { return filepath.Join(cacheDir, "cache") }

	// Manually write an expired cache entry (25 min old)
	expired := time.Now().Add(-25 * time.Minute).Unix()
	data := fmt.Sprintf("%d\n%s\n", expired, "old msg")
	if err := os.WriteFile(cachePath(), []byte(data), 0644); err != nil {
		t.Fatalf("failed to write expired cache: %v", err)
	}

	msg := readCachedVersion()
	if msg != "" {
		t.Fatalf("expected expired cache to return empty, got %q", msg)
	}
}

func TestFetchLatestVersion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := GitHubRelease{TagName: "v1.2.3"}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := server.Client()
	version, err := fetchLatestVersionFromURL(server.URL, client)
	if err != nil {
		t.Fatalf("fetchLatestVersion failed: %v", err)
	}
	if version != "1.2.3" {
		t.Fatalf("expected 1.2.3, got %q", version)
	}
}

func TestCheckUpdate_UpToDate(t *testing.T) {
	orig := cachePath
	origFetch := fetchLatestVersion
	t.Cleanup(func() {
		cachePath = orig
		fetchLatestVersion = origFetch
	})

	cacheDir := t.TempDir()
	cachePath = func() string { return filepath.Join(cacheDir, "cache") }
	fetchLatestVersion = func(*http.Client) (string, error) { return "1.0.0", nil }

	msg := CheckUpdate("1.0.0", nil)
	if msg != "" {
		t.Fatalf("expected no update when versions match, got %q", msg)
	}
}

func TestCheckUpdate_NewVersionAvailable(t *testing.T) {
	orig := cachePath
	origFetch := fetchLatestVersion
	t.Cleanup(func() {
		cachePath = orig
		fetchLatestVersion = origFetch
	})

	cacheDir := t.TempDir()
	cachePath = func() string { return filepath.Join(cacheDir, "cache") }
	fetchLatestVersion = func(*http.Client) (string, error) { return "2.0.0", nil }

	msg := CheckUpdate("1.0.0", nil)
	if msg == "" {
		t.Fatal("expected update message")
	}
	if !strings.Contains(msg, "2.0.0") {
		t.Fatalf("expected new version in message, got %q", msg)
	}
}

func TestCheckUpdate_CachesResult(t *testing.T) {
	orig := cachePath
	origFetch := fetchLatestVersion
	t.Cleanup(func() {
		cachePath = orig
		fetchLatestVersion = origFetch
	})

	cacheDir := t.TempDir()
	cachePath = func() string { return filepath.Join(cacheDir, "cache") }
	fetchLatestVersion = func(*http.Client) (string, error) { return "2.0.0", nil }

	// First call — fetches
	msg1 := CheckUpdate("1.0.0", nil)
	if msg1 == "" {
		t.Fatal("expected update message")
	}

	// Second call — should use cache, not call fetchLatestVersion
	fetchLatestVersion = func(*http.Client) (string, error) {
		t.Fatal("fetchLatestVersion should not be called a second time")
		return "", nil
	}
	msg2 := CheckUpdate("1.0.0", nil)
	if msg2 != msg1 {
		t.Fatalf("expected cached message, got %q", msg2)
	}
}

func TestCheckUpdate_FetchError_ReturnsEmpty(t *testing.T) {
	orig := cachePath
	origFetch := fetchLatestVersion
	t.Cleanup(func() {
		cachePath = orig
		fetchLatestVersion = origFetch
	})

	cacheDir := t.TempDir()
	cachePath = func() string { return filepath.Join(cacheDir, "cache") }
	fetchLatestVersion = func(*http.Client) (string, error) { return "", fmt.Errorf("network error") }

	msg := CheckUpdate("1.0.0", nil)
	if msg != "" {
		t.Fatalf("expected empty on fetch error, got %q", msg)
	}
}
