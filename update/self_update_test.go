package update

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// testChecker creates a Checker with the given fetchFunc and a temp cache dir.
func testChecker(t *testing.T, fetchFunc func(*http.Client) (string, error)) *Checker {
	t.Helper()
	cacheDir := t.TempDir()
	return &Checker{
		fetchLatestVersion: fetchFunc,
		cachePath:          func() string { return filepath.Join(cacheDir, "cache") },
		signingPublicKey:   defaultSigningPublicKey,
	}
}

// testCheckerWithKey creates a Checker with the given fetch func and signing key.
func testCheckerWithKey(t *testing.T, fetchFunc func(*http.Client) (string, error), keyFunc func() (ed25519.PublicKey, error)) *Checker {
	t.Helper()
	cacheDir := t.TempDir()
	return &Checker{
		fetchLatestVersion: fetchFunc,
		cachePath:          func() string { return filepath.Join(cacheDir, "cache") },
		signingPublicKey:   keyFunc,
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
		{current: "1.0.0-rc.1", latest: "1.0.0", expect: -1},
		{current: "1.0.0", latest: "1.0.0-rc.1", expect: 1},
		{current: "1.0.0-alpha.1", latest: "1.0.0-alpha.2", expect: -1},
		{current: "1.0.0-alpha.2", latest: "1.0.0-alpha.10", expect: -1},
		{current: "1.0.0-alpha", latest: "1.0.0-alpha.1", expect: -1},
		{current: "v1.2.3", latest: "1.2.3", expect: 0},
		{current: "1.2.3+build.1", latest: "1.2.3+build.2", expect: 0},
		{current: "1.2", latest: "1.2.0", expect: 0},
		{current: "1.2", latest: "1.2.1", expect: -1},
		{current: "bad", latest: "1.0.0", expect: 1},
		{current: "1.0.0", latest: "bad", expect: -1},
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

func TestWindowsCmdPathUsesSystemRoot(t *testing.T) {
	got := windowsCmdPath(`C:\Windows`)
	want := filepath.Join(`C:\Windows`, "System32", "cmd.exe")
	if got != want {
		t.Fatalf("windowsCmdPath()=%q want %q", got, want)
	}
}

func TestWindowsCmdPathFallsBackToSystemDirectory(t *testing.T) {
	if got := windowsCmdPath(""); got != `C:\Windows\System32\cmd.exe` {
		t.Fatalf("windowsCmdPath()=%q", got)
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

func TestWindowsBatchPath_PercentEscaping(t *testing.T) {
	got := windowsBatchPath(`C:\Users\100%25\motd.exe`)
	if !strings.Contains(got, `%%`) {
		t.Fatalf("expected %% to be escaped, got %q", got)
	}
}

func TestWindowsBatchPath_CaretEscaping(t *testing.T) {
	got := windowsBatchPath(`C:\test\file^name.exe`)
	if !strings.Contains(got, `^^`) {
		t.Fatalf("expected ^ to be escaped, got %q", got)
	}
}

func TestWindowsBatchPath_AmpersandEscaping(t *testing.T) {
	got := windowsBatchPath(`C:\test\file&name.exe`)
	if !strings.Contains(got, `^&`) {
		t.Fatalf("expected & to be escaped, got %q", got)
	}
}

func TestWindowsBatchPath_PipeEscaping(t *testing.T) {
	got := windowsBatchPath(`C:\test\file|name.exe`)
	if !strings.Contains(got, `^|`) {
		t.Fatalf("expected | to be escaped, got %q", got)
	}
}

func TestWindowsBatchPath_AngleBracketEscaping(t *testing.T) {
	got := windowsBatchPath(`C:\test\<file>.exe`)
	if !strings.Contains(got, `^<`) || !strings.Contains(got, `^>`) {
		t.Fatalf("expected angle brackets to be escaped, got %q", got)
	}
}

func TestWindowsBatchPath_ParenthesisEscaping(t *testing.T) {
	got := windowsBatchPath(`C:\test\file(1).exe`)
	if !strings.Contains(got, `^(`) || !strings.Contains(got, `^)`) {
		t.Fatalf("expected parentheses to be escaped, got %q", got)
	}
}

func TestWindowsBatchPath_MultipleMetacharacters(t *testing.T) {
	got := windowsBatchPath(`C:\test\100% & (x).exe`)
	if !strings.Contains(got, `%%`) {
		t.Fatalf("expected %% to be escaped, got %q", got)
	}
	if !strings.Contains(got, `^&`) {
		t.Fatalf("expected & to be escaped, got %q", got)
	}
	if !strings.Contains(got, `^(`) || !strings.Contains(got, `^)`) {
		t.Fatalf("expected parentheses to be escaped, got %q", got)
	}
}

func TestWindowsBatchPath_NoEscapingForNormalPath(t *testing.T) {
	if got := windowsBatchPath(`C:\Windows\System32\cmd.exe`); got != `C:\Windows\System32\cmd.exe` {
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

func TestCheckWriteAccess_WritableDir(t *testing.T) {
	tempDir := t.TempDir()
	execPath := filepath.Join(tempDir, "motd")
	if err := checkWriteAccess(execPath); err != nil {
		t.Fatalf("expected write access to temp dir, got: %v", err)
	}
}

func TestDefaultCachePath_CreatesDirectory(t *testing.T) {
	path := defaultCachePath()
	if path == "" {
		t.Skip("UserCacheDir not available")
	}
	dir := filepath.Dir(path)
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("cache dir does not exist after defaultCachePath(): %v", err)
	}
}

func TestWriteAndReadCachedVersion(t *testing.T) {
	ch := testChecker(t, func(*http.Client) (string, error) { return "", nil })

	// Write a latest version
	ch.writeCachedVersion("2.0.0")

	// Read it back (within cache interval)
	latest := ch.readCachedVersion()
	if latest != "2.0.0" {
		t.Fatalf("expected cached latest version, got %q", latest)
	}
}

func TestCachedVersionExpires(t *testing.T) {
	ch := testChecker(t, func(*http.Client) (string, error) { return "", nil })

	// Manually write an expired cache entry (25 min old)
	expired := time.Now().Add(-25 * time.Minute).Unix()
	entry := cacheEntry{CheckedAt: expired, Latest: "2.0.0"}
	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("failed to marshal cache entry: %v", err)
	}
	if err := os.WriteFile(ch.cachePath(), []byte(data), 0644); err != nil {
		t.Fatalf("failed to write expired cache: %v", err)
	}

	latest := ch.readCachedVersion()
	if latest != "" {
		t.Fatalf("expected expired cache to return empty, got %q", latest)
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
	callCount := 0
	ch := testChecker(t, func(*http.Client) (string, error) {
		callCount++
		return "1.0.0", nil
	})
	msg := ch.CheckUpdate("1.0.0", nil)
	if msg != "" {
		t.Fatalf("expected no update when versions match, got %q", msg)
	}
	if callCount != 1 {
		t.Fatalf("expected 1 fetch call, got %d", callCount)
	}
	// Second call should use cache
	msg = ch.CheckUpdate("1.0.0", nil)
	if msg != "" {
		t.Fatalf("expected no update from cache, got %q", msg)
	}
	if callCount != 1 {
		t.Fatalf("expected fetch to not be called again, got %d calls", callCount)
	}
}

func TestCheckUpdate_NewVersionAvailable(t *testing.T) {
	ch := testChecker(t, func(*http.Client) (string, error) { return "2.0.0", nil })
	msg := ch.CheckUpdate("1.0.0", nil)
	if msg == "" {
		t.Fatal("expected update message")
	}
	if !strings.Contains(msg, "2.0.0") {
		t.Fatalf("expected new version in message, got %q", msg)
	}
}

func TestCheckUpdate_CachesResult(t *testing.T) {
	callCount := 0
	ch := testChecker(t, func(*http.Client) (string, error) {
		callCount++
		return "2.0.0", nil
	})

	// First call — fetches
	msg1 := ch.CheckUpdate("1.0.0", nil)
	if msg1 == "" {
		t.Fatal("expected update message")
	}
	if callCount != 1 {
		t.Fatalf("expected 1 fetch call, got %d", callCount)
	}

	// Second call — should use cache, not call fetchLatestVersion
	msg2 := ch.CheckUpdate("1.0.0", nil)
	if msg2 != msg1 {
		t.Fatalf("expected cached message, got %q", msg2)
	}
	if callCount != 1 {
		t.Fatalf("expected fetch to not be called again, got %d calls", callCount)
	}
}

func TestCheckUpdate_CachesUptodateResult(t *testing.T) {
	callCount := 0
	ch := testChecker(t, func(*http.Client) (string, error) {
		callCount++
		return "1.0.0", nil
	})

	// First call — fetches, discovers we're up-to-date
	msg := ch.CheckUpdate("1.0.0", nil)
	if msg != "" {
		t.Fatalf("expected empty for uptodate, got %q", msg)
	}
	if callCount != 1 {
		t.Fatalf("expected 1 fetch call, got %d", callCount)
	}

	// Second call — should use cache, not call fetchLatestVersion
	msg = ch.CheckUpdate("1.0.0", nil)
	if msg != "" {
		t.Fatalf("expected empty for uptodate (cached), got %q", msg)
	}
	if callCount != 1 {
		t.Fatalf("expected fetch to not be called again, got %d calls", callCount)
	}
}

func TestCheckUpdate_CachedLatestRecomparesCurrentVersion(t *testing.T) {
	callCount := 0
	ch := testChecker(t, func(*http.Client) (string, error) {
		callCount++
		return "1.7.3", nil
	})

	msg := ch.CheckUpdate("1.7.1", nil)
	if msg == "" {
		t.Fatal("expected update message")
	}
	if !strings.Contains(msg, "1.7.1") || !strings.Contains(msg, "1.7.3") {
		t.Fatalf("expected current and latest versions in message, got %q", msg)
	}
	if callCount != 1 {
		t.Fatalf("expected 1 fetch call, got %d", callCount)
	}

	msg = ch.CheckUpdate("1.7.3", nil)
	if msg != "" {
		t.Fatalf("expected no update after current version catches up to cached latest, got %q", msg)
	}
	if callCount != 1 {
		t.Fatalf("expected cached latest to avoid another fetch, got %d calls", callCount)
	}
}

func TestCheckUpdate_TransitionFromUptodateToNewVersion(t *testing.T) {
	callCount := 0
	ch := testChecker(t, func(*http.Client) (string, error) {
		callCount++
		if callCount == 1 {
			return "1.0.0", nil
		}
		return "2.0.0", nil
	})

	// First call — uptodate
	msg := ch.CheckUpdate("1.0.0", nil)
	if msg != "" {
		t.Fatalf("expected empty for uptodate, got %q", msg)
	}
	if callCount != 1 {
		t.Fatalf("expected 1 fetch call, got %d", callCount)
	}

	// Force cache expiry for the next read
	// write an expired entry so we fetch again
	ch.writeCachedVersion("")
	// Now set up to return new version
	origFetch := ch.fetchLatestVersion
	ch.fetchLatestVersion = func(*http.Client) (string, error) { return "2.0.0", nil }

	msg = ch.CheckUpdate("1.0.0", nil)
	if msg == "" {
		t.Fatal("expected update message")
	}
	ch.fetchLatestVersion = origFetch
}

func TestCheckUpdate_FetchError_ReturnsEmpty(t *testing.T) {
	ch := testChecker(t, func(*http.Client) (string, error) { return "", fmt.Errorf("network error") })
	msg := ch.CheckUpdate("1.0.0", nil)
	if msg != "" {
		t.Fatalf("expected empty on fetch error, got %q", msg)
	}
}

func TestGetChecksumsSignatureURL_Found(t *testing.T) {
	release := &GitHubRelease{
		Assets: []struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
		}{
			{Name: "checksums.txt", URL: "https://example.com/checksums.txt"},
			{Name: "checksums.txt.sig", URL: "https://example.com/checksums.txt.sig"},
		},
	}
	url := getChecksumsSignatureURL(release)
	if url != "https://example.com/checksums.txt.sig" {
		t.Fatalf("expected sig URL, got %q", url)
	}
}

func TestGetChecksumsSignatureURL_NotFound(t *testing.T) {
	release := &GitHubRelease{
		Assets: []struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
		}{
			{Name: "checksums.txt", URL: "https://example.com/checksums.txt"},
		},
	}
	url := getChecksumsSignatureURL(release)
	if url != "" {
		t.Fatalf("expected empty URL, got %q", url)
	}
}

func TestVerifyChecksumsSignature_Valid(t *testing.T) {
	pubKey, privKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate test key: %v", err)
	}

	ch := testCheckerWithKey(t, func(*http.Client) (string, error) { return "", nil },
		func() (ed25519.PublicKey, error) { return pubKey, nil })

	data := []byte("sha256 abc  motd-linux-amd64\n")
	sig := ed25519.Sign(privKey, data)

	if err := ch.verifyChecksumsSignature(data, sig); err != nil {
		t.Fatalf("expected valid signature, got: %v", err)
	}
}

func TestVerifyChecksumsSignature_Invalid(t *testing.T) {
	pubKey, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate test key: %v", err)
	}

	ch := testCheckerWithKey(t, func(*http.Client) (string, error) { return "", nil },
		func() (ed25519.PublicKey, error) { return pubKey, nil })

	data := []byte("sha256 abc  motd-linux-amd64\n")
	_, wrongKey, _ := ed25519.GenerateKey(rand.Reader)
	sig := ed25519.Sign(wrongKey, data)

	if err := ch.verifyChecksumsSignature(data, sig); err == nil {
		t.Fatal("expected signature verification to fail")
	}
}

func TestVerifyChecksumsSignature_TamperedData(t *testing.T) {
	pubKey, privKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate test key: %v", err)
	}

	ch := testCheckerWithKey(t, func(*http.Client) (string, error) { return "", nil },
		func() (ed25519.PublicKey, error) { return pubKey, nil })

	data := []byte("sha256 abc  motd-linux-amd64\n")
	sig := ed25519.Sign(privKey, data)

	tampered := []byte("sha256 xyz  motd-linux-amd64\n")
	if err := ch.verifyChecksumsSignature(tampered, sig); err == nil {
		t.Fatal("expected signature verification to fail on tampered data")
	}
}

func TestDownloadChecksumsSignature_Success(t *testing.T) {
	sigData := []byte("fake-signature-bytes")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(sigData)
	}))
	defer server.Close()

	release := &GitHubRelease{
		Assets: []struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
		}{
			{Name: "checksums.txt.sig", URL: server.URL},
		},
	}

	client := server.Client()
	got, err := downloadChecksumsSignature(release, client)
	if err != nil {
		t.Fatalf("downloadChecksumsSignature failed: %v", err)
	}
	if string(got) != string(sigData) {
		t.Fatalf("expected %q, got %q", sigData, got)
	}
}

func TestRealPublicKey_IsNotZero(t *testing.T) {
	pubKey, err := defaultSigningPublicKey()
	if err != nil {
		t.Fatalf("defaultSigningPublicKey failed: %v", err)
	}
	if isZeroKey(pubKey) {
		t.Fatal("expected real public key, got zero key — checksumsPublicKeyHex may still be a placeholder")
	}
	if len(pubKey) != ed25519.PublicKeySize {
		t.Fatalf("expected %d-byte public key, got %d", ed25519.PublicKeySize, len(pubKey))
	}
}

func TestDownloadChecksumsSignature_Missing(t *testing.T) {
	release := &GitHubRelease{
		Assets: []struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
		}{
			{Name: "checksums.txt", URL: "https://example.com/checksums.txt"},
		},
	}
	_, err := downloadChecksumsSignature(release, nil)
	if !errors.Is(err, errMissingSig) {
		t.Fatalf("expected errMissingSig, got: %v", err)
	}
}

func TestDownloadChecksumsSignature_Non200Status(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	release := &GitHubRelease{
		Assets: []struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
		}{
			{Name: "checksums.txt.sig", URL: server.URL},
		},
	}

	client := server.Client()
	_, err := downloadChecksumsSignature(release, client)
	if err == nil {
		t.Fatal("expected error for non-200 status")
	}
}

func TestVerifyChecksumsSignature_WrongLengthSig(t *testing.T) {
	pubKey, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate test key: %v", err)
	}

	ch := testCheckerWithKey(t, func(*http.Client) (string, error) { return "", nil },
		func() (ed25519.PublicKey, error) { return pubKey, nil })

	data := []byte("sha256 abc  motd-linux-amd64\n")

	tests := []struct {
		name string
		sig  []byte
	}{
		{name: "nil signature", sig: nil},
		{name: "empty signature", sig: []byte{}},
		{name: "short signature", sig: make([]byte, ed25519.SignatureSize-1)},
		{name: "long signature", sig: make([]byte, ed25519.SignatureSize+1)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ch.verifyChecksumsSignature(data, tt.sig); err == nil {
				t.Fatal("expected error for wrong-length signature")
			}
		})
	}
}

func TestIsZeroKey(t *testing.T) {
	tests := []struct {
		name string
		key  ed25519.PublicKey
		want bool
	}{
		{name: "nil key", key: nil, want: true},
		{name: "empty key", key: ed25519.PublicKey{}, want: true},
		{name: "32 zero bytes", key: make(ed25519.PublicKey, ed25519.PublicKeySize), want: true},
		{name: "non-zero key", key: bytes.Repeat([]byte{0x01}, ed25519.PublicKeySize), want: false},
		{name: "partial zero (last byte non-zero)", key: func() ed25519.PublicKey {
			k := make(ed25519.PublicKey, ed25519.PublicKeySize)
			k[ed25519.PublicKeySize-1] = 0x01
			return k
		}(), want: false},
		{name: "partial zero (middle byte non-zero)", key: func() ed25519.PublicKey {
			k := make(ed25519.PublicKey, ed25519.PublicKeySize)
			k[16] = 0x01
			return k
		}(), want: false},
		{name: "single byte key", key: ed25519.PublicKey{0x00}, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isZeroKey(tt.key); got != tt.want {
				t.Fatalf("isZeroKey(%v) = %v, want %v", tt.key, got, tt.want)
			}
		})
	}
}

func TestParseChecksumLine_NoStarPrefix(t *testing.T) {
	checksum, filename, ok := parseChecksumLine("abc123  motd-linux-amd64")
	if !ok {
		t.Fatal("expected checksum line to parse")
	}
	if checksum != "abc123" || filename != "motd-linux-amd64" {
		t.Fatalf("unexpected parse result: %q %q", checksum, filename)
	}
}

func TestParseChecksumLine_TooFewFields(t *testing.T) {
	_, _, ok := parseChecksumLine("abc123")
	if ok {
		t.Fatal("expected parse to fail with only one field")
	}
}

func TestParseChecksumLine_EmptyLine(t *testing.T) {
	_, _, ok := parseChecksumLine("")
	if ok {
		t.Fatal("expected parse to fail on empty line")
	}
}

func TestParseChecksumLine_WhitespaceOnly(t *testing.T) {
	_, _, ok := parseChecksumLine("   ")
	if ok {
		t.Fatal("expected parse to fail on whitespace-only line")
	}
}

func TestGetChecksums_MissingAsset(t *testing.T) {
	release := &GitHubRelease{
		Assets: []struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
		}{},
	}
	_, _, err := getChecksums(release, nil)
	if err == nil {
		t.Fatal("expected error when checksums.txt is missing")
	}
}

func TestGetChecksums_Non200Status(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	release := &GitHubRelease{
		Assets: []struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
		}{
			{Name: "checksums.txt", URL: server.URL},
		},
	}
	client := server.Client()
	_, _, err := getChecksums(release, client)
	if err == nil || !strings.Contains(err.Error(), "404") {
		t.Fatalf("expected error mentioning 404, got: %v", err)
	}
}

func TestGetChecksums_EmptyBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	release := &GitHubRelease{
		Assets: []struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
		}{
			{Name: "checksums.txt", URL: server.URL},
		},
	}
	client := server.Client()
	checksums, body, err := getChecksums(release, client)
	if err != nil {
		t.Fatalf("getChecksums failed on empty body: %v", err)
	}
	if len(checksums) != 0 {
		t.Fatalf("expected empty checksums map, got %v", checksums)
	}
	if len(body) != 0 {
		t.Fatalf("expected empty body bytes, got %d bytes", len(body))
	}
}

func TestGetChecksums_MalformedLines(t *testing.T) {
	body := "abc123  motd-linux-amd64\n\ndef456\n\n  \nghi789  motd-darwin-amd64\n"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(body))
	}))
	defer server.Close()

	release := &GitHubRelease{
		Assets: []struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
		}{
			{Name: "checksums.txt", URL: server.URL},
		},
	}
	client := server.Client()
	checksums, rawBody, err := getChecksums(release, client)
	if err != nil {
		t.Fatalf("getChecksums failed: %v", err)
	}
	if len(checksums) != 2 {
		t.Fatalf("expected 2 valid checksum entries, got %d: %v", len(checksums), checksums)
	}
	if checksums["motd-linux-amd64"] != "abc123" {
		t.Fatalf("expected abc123 for linux, got %q", checksums["motd-linux-amd64"])
	}
	if checksums["motd-darwin-amd64"] != "ghi789" {
		t.Fatalf("expected ghi789 for darwin, got %q", checksums["motd-darwin-amd64"])
	}
	if string(rawBody) != body {
		t.Fatalf("raw body mismatch: got %q, want %q", string(rawBody), body)
	}
}

func TestGetChecksums_Success(t *testing.T) {
	body := "abc123  motd-linux-amd64\ndef456  motd-darwin-arm64\n"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(body))
	}))
	defer server.Close()

	release := &GitHubRelease{
		Assets: []struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
		}{
			{Name: "checksums.txt", URL: server.URL},
		},
	}
	client := server.Client()
	checksums, raw, err := getChecksums(release, client)
	if err != nil {
		t.Fatalf("getChecksums failed: %v", err)
	}
	if len(checksums) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(checksums))
	}
	if checksums["motd-linux-amd64"] != "abc123" {
		t.Fatalf("unexpected checksum for linux: %q", checksums["motd-linux-amd64"])
	}
	if checksums["motd-darwin-arm64"] != "def456" {
		t.Fatalf("unexpected checksum for darwin: %q", checksums["motd-darwin-arm64"])
	}
	if string(raw) != body {
		t.Fatalf("raw body mismatch: got %q, want %q", string(raw), body)
	}
}

func TestGetLatestRelease_OversizedResponse(t *testing.T) {
	payload := make([]byte, maxReleaseJSONSize+1)
	for i := range payload {
		payload[i] = ' '
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(payload)
	}))
	defer server.Close()

	client := server.Client()
	// We need to call the underlying logic. getLatestRelease uses a fixed URL,
	// so we test via fetchLatestVersionFromURL which also reads JSON.
	_, err := fetchLatestVersionFromURL(server.URL, client)
	if err == nil || !strings.Contains(err.Error(), "too large") {
		t.Fatalf("expected 'too large' error, got: %v", err)
	}
}

func TestGetChecksums_OversizedResponse(t *testing.T) {
	payload := make([]byte, maxChecksumsSize+1)
	for i := range payload {
		payload[i] = 'a'
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(payload)
	}))
	defer server.Close()

	release := &GitHubRelease{
		Assets: []struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
		}{
			{Name: "checksums.txt", URL: server.URL},
		},
	}
	client := server.Client()
	_, _, err := getChecksums(release, client)
	if err == nil || !strings.Contains(err.Error(), "too large") {
		t.Fatalf("expected 'too large' error, got: %v", err)
	}
}

func TestGetChecksumsSignature_OversizedResponse(t *testing.T) {
	payload := make([]byte, maxChecksumsSize+1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(payload)
	}))
	defer server.Close()

	release := &GitHubRelease{
		Assets: []struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
		}{
			{Name: "checksums.txt.sig", URL: server.URL},
		},
	}
	client := server.Client()
	_, err := downloadChecksumsSignature(release, client)
	if err == nil || !strings.Contains(err.Error(), "too large") {
		t.Fatalf("expected 'too large' error, got: %v", err)
	}
}

func TestDownloadBinary_OversizedResponse(t *testing.T) {
	payload := make([]byte, maxBinarySize+1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(payload)
	}))
	defer server.Close()

	client := server.Client()
	_, err := downloadBinary(server.URL, "motd-linux-amd64", map[string]string{"motd-linux-amd64": "abc"}, t.TempDir(), client)
	if err == nil || !strings.Contains(err.Error(), "too large") {
		t.Fatalf("expected 'too large' error, got: %v", err)
	}
}

func TestGetLatestRelease_NormalSizedResponse(t *testing.T) {
	release := GitHubRelease{TagName: "v1.2.3"}
	var buf bytes.Buffer
	json.NewEncoder(&buf).Encode(release)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(buf.Bytes())
	}))
	defer server.Close()

	client := server.Client()
	version, err := fetchLatestVersionFromURL(server.URL, client)
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
	if version != "1.2.3" {
		t.Fatalf("expected 1.2.3, got %q", version)
	}
}

func TestDownloadBinary_NormalSizedResponse(t *testing.T) {
	payload := []byte("small binary")
	checksum := fmt.Sprintf("%x", sha256.Sum256(payload))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(payload)
	}))
	defer server.Close()

	client := server.Client()
	path, err := downloadBinary(server.URL, "motd-linux-amd64", map[string]string{"motd-linux-amd64": checksum}, t.TempDir(), client)
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
	defer os.Remove(path)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read downloaded file: %v", err)
	}
	if string(data) != string(payload) {
		t.Fatalf("expected %q, got %q", payload, data)
	}
}

func TestDownloadBinary_CleansUpOnChecksumMismatch(t *testing.T) {
	payload := []byte("some binary data")
	checksum := fmt.Sprintf("%x", sha256.Sum256(payload))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(payload)
	}))
	defer server.Close()

	dir := t.TempDir()
	client := server.Client()

	// Use wrong filename to trigger "no checksum found" error.
	_, err := downloadBinary(server.URL, "wrong-filename", map[string]string{"motd-linux-amd64": checksum}, dir, client)
	if err == nil {
		t.Fatal("expected error for missing checksum")
	}

	// Verify no temp files remain in the directory.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("failed to read dir: %v", err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "motd-update-") {
			t.Fatalf("found orphaned temp file after failed download: %s", e.Name())
		}
	}
}

func TestPerformUpdateIntegration_HappyPath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("performUpdate integration test uses Unix-specific binary replacement path")
	}

	pubKey, privKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate test key: %v", err)
	}

	binaryPayload := []byte("mock motd binary")
	binaryChecksum := fmt.Sprintf("%x", sha256.Sum256(binaryPayload))
	checksumsContent := fmt.Sprintf("%s  %s\n", binaryChecksum, "motd-linux-amd64")
	sig := ed25519.Sign(privKey, []byte(checksumsContent))

	// Create a release with a mock GitHub release JSON endpoint plus asset URLs.
	release := &GitHubRelease{
		TagName: "v1.2.3",
		Assets: []struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
		}{
			{Name: "checksums.txt", URL: ""},
			{Name: "checksums.txt.sig", URL: ""},
			{Name: "motd-linux-amd64", URL: ""},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/checksums.txt":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(checksumsContent))
		case "/checksums.txt.sig":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(sig)
		case "/binary":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(binaryPayload)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	release.Assets[0].URL = server.URL + "/checksums.txt"
	release.Assets[1].URL = server.URL + "/checksums.txt.sig"
	release.Assets[2].URL = server.URL + "/binary"

	ch := &Checker{
		fetchLatestVersion: func(*http.Client) (string, error) { return "", nil },
		cachePath:          func() string { return filepath.Join(t.TempDir(), "cache") },
		signingPublicKey:   func() (ed25519.PublicKey, error) { return pubKey, nil },
	}

	client := server.Client()

	// Step 1: Download and parse checksums
	checksums, checksumsData, err := getChecksums(release, client)
	if err != nil {
		t.Fatalf("getChecksums failed: %v", err)
	}
	if len(checksums) != 1 {
		t.Fatalf("expected 1 checksum entry, got %d", len(checksums))
	}
	if checksums["motd-linux-amd64"] != binaryChecksum {
		t.Fatalf("checksum mismatch: got %q, want %q", checksums["motd-linux-amd64"], binaryChecksum)
	}

	// Step 2: Download signature
	sigData, err := downloadChecksumsSignature(release, client)
	if err != nil {
		t.Fatalf("downloadChecksumsSignature failed: %v", err)
	}

	// Step 3: Verify signature
	if err := ch.verifyChecksumsSignature(checksumsData, sigData); err != nil {
		t.Fatalf("verifyChecksumsSignature failed: %v", err)
	}

	// Step 4: Download binary and verify checksum
	tempDir := t.TempDir()
	binPath, err := downloadBinary(release.Assets[2].URL, "motd-linux-amd64", checksums, tempDir, client)
	if err != nil {
		t.Fatalf("downloadBinary failed: %v", err)
	}
	defer os.Remove(binPath)

	downloaded, err := os.ReadFile(binPath)
	if err != nil {
		t.Fatalf("failed to read downloaded binary: %v", err)
	}
	if string(downloaded) != string(binaryPayload) {
		t.Fatalf("binary content mismatch: got %q, want %q", downloaded, binaryPayload)
	}
}

func TestPerformUpdateIntegration_MissingSignature(t *testing.T) {
	release := &GitHubRelease{
		TagName: "v1.2.3",
		Assets: []struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
		}{
			{Name: "checksums.txt", URL: "http://example.com/checksums.txt"},
		},
	}
	_, err := downloadChecksumsSignature(release, nil)
	if !errors.Is(err, errMissingSig) {
		t.Fatalf("expected errMissingSig, got: %v", err)
	}
}

func TestPerformUpdateIntegration_InvalidSignature(t *testing.T) {
	pubKey, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate test key: %v", err)
	}

	_, wrongKey, _ := ed25519.GenerateKey(rand.Reader)
	checksumsContent := "abc123  motd-linux-amd64\n"
	sig := ed25519.Sign(wrongKey, []byte(checksumsContent))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(sig)
	}))
	defer server.Close()

	release := &GitHubRelease{
		TagName: "v1.2.3",
		Assets: []struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
		}{
			{Name: "checksums.txt", URL: "http://example.com/checksums.txt"},
			{Name: "checksums.txt.sig", URL: server.URL},
		},
	}

	ch := &Checker{
		fetchLatestVersion: func(*http.Client) (string, error) { return "", nil },
		cachePath:          func() string { return filepath.Join(t.TempDir(), "cache") },
		signingPublicKey:   func() (ed25519.PublicKey, error) { return pubKey, nil },
	}

	client := server.Client()
	sigData, err := downloadChecksumsSignature(release, client)
	if err != nil {
		t.Fatalf("downloadChecksumsSignature failed: %v", err)
	}

	if err := ch.verifyChecksumsSignature([]byte(checksumsContent), sigData); err == nil {
		t.Fatal("expected signature verification to fail with wrong key")
	}
}

func TestPerformUpdateIntegration_ChecksumMismatch(t *testing.T) {
	binaryPayload := []byte("mock binary")
	actualChecksum := fmt.Sprintf("%x", sha256.Sum256(binaryPayload))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(binaryPayload)
	}))
	defer server.Close()

	client := server.Client()
	// Deliberately wrong checksum
	_, err := downloadBinary(server.URL, "motd-linux-amd64",
		map[string]string{"motd-linux-amd64": "1111111111111111111111111111111111111111111111111111111111111111"},
		t.TempDir(), client)
	if err == nil || !strings.Contains(err.Error(), "checksum verification failed") {
		t.Fatalf("expected checksum verification failure, got: %v", err)
	}
	// Verify actual checksum works (sanity check)
	path, err := downloadBinary(server.URL, "motd-linux-amd64",
		map[string]string{"motd-linux-amd64": actualChecksum}, t.TempDir(), client)
	if err != nil {
		t.Fatalf("expected success with correct checksum, got: %v", err)
	}
	os.Remove(path)
}

func TestDownloadBinary_CleansUpOnHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	dir := t.TempDir()
	client := server.Client()
	_, err := downloadBinary(server.URL, "motd-linux-amd64", map[string]string{"motd-linux-amd64": "abc"}, dir, client)
	if err == nil {
		t.Fatal("expected error for non-200 status")
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("failed to read dir: %v", err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "motd-update-") {
			t.Fatalf("found orphaned temp file after HTTP error: %s", e.Name())
		}
	}
}
