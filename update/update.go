package update

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"motd/display"
	"motd/util"
)

const (
	maxReleaseJSONSize = 1 << 20   // 1 MB
	maxChecksumsSize   = 64 << 10  // 64 KB
	maxBinarySize      = 100 << 20 // 100 MB
)

type GitHubRelease struct {
	TagName string `json:"tag_name"`
	Name    string `json:"name"`
	Assets  []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
	} `json:"assets"`
}

// checksumsPublicKeyHex is the Ed25519 public key used to verify
// checksums.txt signatures. The corresponding private key is stored
// as a GitHub secret (SIGNING_PRIVATE_KEY) and used during release.
//
// To generate a new key pair:
//
//	openssl genpkey -algorithm ed25519 -out private.pem
//	openssl pkey -in private.pem -pubout -out public.pem
//
// Extract the raw 32-byte public key:
//
//	openssl pkey -in public.pem -pubin -outform DER | tail -c 32 | xxd -p
//
// Replace the placeholder below with the hex-encoded public key.
const checksumsPublicKeyHex = "77c6d63c22d1e65b1793563b48c2ad00117114f581476ced38b3228b02f99831"

var (
	errMissingSig       = errors.New("checksums.txt.sig not found in release assets")
	errInvalidSig       = errors.New("checksums.txt signature verification failed: checksums file has been tampered with")
	errKeyNotConfigured = errors.New("checksums signing key not configured: replace the placeholder in checksumsPublicKeyHex and set SIGNING_PRIVATE_KEY in repo secrets")
)

func defaultSigningPublicKey() (ed25519.PublicKey, error) {
	b, err := hex.DecodeString(checksumsPublicKeyHex)
	if err != nil {
		return nil, fmt.Errorf("invalid checksums public key hex: %w", err)
	}
	if len(b) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("invalid checksums public key length: got %d, want %d", len(b), ed25519.PublicKeySize)
	}
	return ed25519.PublicKey(b), nil
}

// Checker holds the dependencies for checking and performing updates.
// Tests should create isolated Checker values instead of mutating globals.
type Checker struct {
	fetchLatestVersion func(client *http.Client) (string, error)
	cachePath          func() string
	signingPublicKey   func() (ed25519.PublicKey, error)
}

// NewChecker returns a Checker with production defaults.
func NewChecker() *Checker {
	return &Checker{
		fetchLatestVersion: func(client *http.Client) (string, error) {
			return fetchLatestVersionFromURL("https://api.github.com/repos/thewildhive/go-motd/releases/latest", client)
		},
		cachePath:        defaultCachePath,
		signingPublicKey: defaultSigningPublicKey,
	}
}

func HandleSelfUpdate(version string, client *http.Client) {
	NewChecker().HandleSelfUpdate(version, client)
}

func (ch *Checker) HandleSelfUpdate(version string, client *http.Client) {
	force := false
	if len(os.Args) > 2 && os.Args[2] == "--force" {
		force = true
	}

	fmt.Printf("%sChecking for updates...%s\n", display.Cyan, display.Reset)

	release, err := getLatestRelease(client)
	if err != nil {
		fmt.Printf("%sError checking for updates: %v%s\n", display.Red, err, display.Reset)
		os.Exit(1)
	}

	latestVersion := strings.TrimPrefix(release.TagName, "v")
	currentVersion := version

	fmt.Printf("Current version: %s%s%s\n", display.Blue, currentVersion, display.Reset)
	fmt.Printf("Latest version:  %s%s%s\n", display.Blue, latestVersion, display.Reset)

	if !force && CompareVersions(currentVersion, latestVersion) >= 0 {
		fmt.Printf("%sAlready running the latest version!%s\n", display.Green, display.Reset)
		return
	}

	if !force {
		fmt.Printf("\n%sA new version is available!%s\n", display.Yellow, display.Reset)
		fmt.Printf("Update from %s%s%s to %s%s%s?\n", display.Red, currentVersion, display.Reset, display.Green, latestVersion, display.Reset)

		if !isInteractive() {
			fmt.Println("Non-interactive mode detected. Use --force to update without prompt.")
			fmt.Println("Update cancelled.")
			return
		}

		fmt.Print("Update? [y/N]: ")
		var response string
		fmt.Scanln(&response)
		if strings.ToLower(response) != "y" && strings.ToLower(response) != "yes" {
			fmt.Println("Update cancelled.")
			return
		}
	}

	execPath, err := os.Executable()
	if err != nil {
		fmt.Printf("%sError determining executable path: %v%s\n", display.Red, err, display.Reset)
		os.Exit(1)
	}

	// Check write access to the binary directory before downloading.
	if err := checkWriteAccess(execPath); err != nil {
		localBin := filepath.Join(os.Getenv("HOME"), ".local", "bin")
		fmt.Printf("%sCannot write to %s%s\n", display.Red, filepath.Dir(execPath), display.Reset)
		fmt.Printf("Try running with sudo, or install to %s and add it to your PATH.\n", localBin)
		fmt.Printf("  mkdir -p %s\n", localBin)
		fmt.Printf("  cp %s %s/\n", execPath, localBin)
		os.Exit(1)
	}

	fmt.Printf("\n%sUpdating to version %s...%s\n", display.Cyan, latestVersion, display.Reset)

	if err := ch.performUpdate(release, latestVersion, client); err != nil {
		fmt.Printf("%sUpdate failed: %v%s\n", display.Red, err, display.Reset)
		os.Exit(1)
	}

	fmt.Printf("%sSuccessfully updated to version %s!%s\n", display.Green, latestVersion, display.Reset)
}

func getLatestRelease(client *http.Client) (*GitHubRelease, error) {
	url := "https://api.github.com/repos/thewildhive/go-motd/releases/latest"

	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch release info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxReleaseJSONSize))
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}
	if len(body) >= maxReleaseJSONSize {
		return nil, fmt.Errorf("release JSON response too large (max %d bytes)", maxReleaseJSONSize)
	}

	var release GitHubRelease
	if err := json.Unmarshal(body, &release); err != nil {
		return nil, fmt.Errorf("failed to parse release info: %w", err)
	}

	return &release, nil
}

func CompareVersions(current, latest string) int {
	currentParts := strings.Split(current, ".")
	latestParts := strings.Split(latest, ".")

	maxLen := len(currentParts)
	if len(latestParts) > maxLen {
		maxLen = len(latestParts)
	}

	for i := 0; i < maxLen; i++ {
		var currentNum, latestNum int

		if i < len(currentParts) {
			currentNum, _ = strconv.Atoi(currentParts[i])
		}
		if i < len(latestParts) {
			latestNum, _ = strconv.Atoi(latestParts[i])
		}

		if currentNum < latestNum {
			return -1
		} else if currentNum > latestNum {
			return 1
		}
	}

	return 0
}

func (ch *Checker) performUpdate(release *GitHubRelease, version string, client *http.Client) error {
	assetName := getPlatformAssetName()
	if assetName == "" {
		return fmt.Errorf("unsupported platform: %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	var downloadURL string
	for _, asset := range release.Assets {
		if asset.Name == assetName {
			downloadURL = asset.URL
			break
		}
	}

	if downloadURL == "" {
		return fmt.Errorf("could not find release asset for platform %s", assetName)
	}

	checksums, checksumsData, err := getChecksums(release, client)
	if err != nil {
		return fmt.Errorf("failed to get checksums: %w", err)
	}

	sigData, err := downloadChecksumsSignature(release, client)
	if err != nil {
		return fmt.Errorf("failed to get checksums signature: %w", err)
	}
	if err := ch.verifyChecksumsSignature(checksumsData, sigData); err != nil {
		return fmt.Errorf("checksums verification rejected: %w", err)
	}

	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	tempFile, err := downloadBinary(downloadURL, assetName, checksums, filepath.Dir(execPath), client)
	if err != nil {
		return fmt.Errorf("failed to download binary: %w", err)
	}
	removeTemp := true
	defer func() {
		if removeTemp {
			_ = os.Remove(tempFile)
		}
	}()

	backupPath := execPath + ".backup"
	if err := util.CopyFile(execPath, backupPath); err != nil {
		return fmt.Errorf("failed to create backup: %w", err)
	}
	removeBackup := runtime.GOOS != "windows"
	defer func() {
		if removeBackup {
			_ = os.Remove(backupPath)
		}
	}()

	if err := replaceBinary(tempFile, execPath, backupPath); err != nil {
		if rollbackErr := restoreBackup(backupPath, execPath); rollbackErr != nil {
			return fmt.Errorf("update failed and rollback failed: %v (rollback error: %v)", err, rollbackErr)
		}
		return fmt.Errorf("update failed, rolled back to previous version: %w", err)
	}
	if runtime.GOOS == "windows" {
		removeTemp = false
	}

	return nil
}

func getPlatformAssetName() string {
	return platformAssetName(runtime.GOOS, runtime.GOARCH)
}

func platformAssetName(goos, goarch string) string {
	switch goos {
	case "linux":
		switch goarch {
		case "amd64":
			return "motd-linux-amd64"
		case "arm64":
			return "motd-linux-arm64"
		}
	case "darwin":
		switch goarch {
		case "amd64":
			return "motd-darwin-amd64"
		case "arm64":
			return "motd-darwin-arm64"
		}
	case "windows":
		if goarch == "amd64" {
			return "motd-windows-amd64.exe"
		}
	}
	return ""
}

// getChecksums downloads checksums.txt from the release assets, parses it
// into a filename→sha256 map, and returns the map along with the raw bytes
// (which the caller uses for signature verification).
func getChecksums(release *GitHubRelease, client *http.Client) (map[string]string, []byte, error) {
	var checksumsURL string
	for _, asset := range release.Assets {
		if asset.Name == "checksums.txt" {
			checksumsURL = asset.URL
			break
		}
	}

	if checksumsURL == "" {
		return nil, nil, fmt.Errorf("checksums.txt not found in release assets")
	}

	resp, err := client.Get(checksumsURL)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to download checksums: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("checksums download returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxChecksumsSize))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read checksums: %w", err)
	}
	if len(body) >= maxChecksumsSize {
		return nil, nil, fmt.Errorf("checksums file too large (max %d bytes)", maxChecksumsSize)
	}

	checksums := make(map[string]string)
	lines := strings.Split(string(body), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		checksum, filename, ok := parseChecksumLine(line)
		if ok {
			checksums[filename] = checksum
		}
	}

	return checksums, body, nil
}

func parseChecksumLine(line string) (string, string, bool) {
	parts := strings.Fields(line)
	if len(parts) < 2 {
		return "", "", false
	}
	return parts[0], strings.TrimPrefix(parts[1], "*"), true
}

// getChecksumsSignatureURL finds the checksums.txt.sig asset URL in the release.
func getChecksumsSignatureURL(release *GitHubRelease) string {
	for _, asset := range release.Assets {
		if asset.Name == "checksums.txt.sig" {
			return asset.URL
		}
	}
	return ""
}

// downloadChecksumsSignature downloads the Ed25519 signature file for
// checksums.txt from the release assets.
func downloadChecksumsSignature(release *GitHubRelease, client *http.Client) ([]byte, error) {
	sigURL := getChecksumsSignatureURL(release)
	if sigURL == "" {
		return nil, errMissingSig
	}

	resp, err := client.Get(sigURL)
	if err != nil {
		return nil, fmt.Errorf("failed to download checksums signature: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("checksums signature download returned status %d", resp.StatusCode)
	}

	sig, err := io.ReadAll(io.LimitReader(resp.Body, maxChecksumsSize))
	if err != nil {
		return nil, fmt.Errorf("failed to read checksums signature: %w", err)
	}
	if len(sig) >= maxChecksumsSize {
		return nil, fmt.Errorf("checksums signature too large (max %d bytes)", maxChecksumsSize)
	}

	return sig, nil
}

// isZeroKey reports whether pubKey is the all-zero placeholder.
func isZeroKey(pubKey ed25519.PublicKey) bool {
	for _, b := range pubKey {
		if b != 0 {
			return false
		}
	}
	return true
}

// verifyChecksumsSignature verifies the Ed25519 signature of checksums.txt
// data using the embedded public key.
func (ch *Checker) verifyChecksumsSignature(data, sig []byte) error {
	pubKey, err := ch.signingPublicKey()
	if err != nil {
		return err
	}
	if isZeroKey(pubKey) {
		return errKeyNotConfigured
	}
	if !ed25519.Verify(pubKey, data, sig) {
		return errInvalidSig
	}
	return nil
}

func downloadBinary(url, filename string, checksums map[string]string, tempDir string, client *http.Client) (string, error) {
	tempFile, err := os.CreateTemp(tempDir, "motd-update-*.tmp")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tempFile.Name()
	removeTemp := true
	defer func() {
		tempFile.Close()
		if removeTemp {
			os.Remove(tmpPath)
		}
	}()

	resp, err := client.Get(url)
	if err != nil {
		return "", fmt.Errorf("failed to download binary: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download returned status %d", resp.StatusCode)
	}

	hasher := sha256.New()
	multiWriter := io.MultiWriter(tempFile, hasher)

	limited := io.LimitReader(resp.Body, maxBinarySize+1)
	written, err := io.Copy(multiWriter, limited)
	if err != nil {
		return "", fmt.Errorf("failed to save binary: %w", err)
	}
	if written > maxBinarySize {
		return "", fmt.Errorf("binary download too large (max %d bytes)", maxBinarySize)
	}

	downloadedChecksum := fmt.Sprintf("%x", hasher.Sum(nil))
	expectedChecksum, exists := checksums[filename]
	if !exists {
		return "", fmt.Errorf("no checksum found for %s", filename)
	}

	if downloadedChecksum != expectedChecksum {
		return "", fmt.Errorf("checksum verification failed: expected %s, got %s", expectedChecksum, downloadedChecksum)
	}

	removeTemp = false
	return tmpPath, nil
}

func windowsCmdPath(systemRoot string) string {
	if systemRoot == "" {
		return `C:\Windows\System32\cmd.exe`
	}
	return filepath.Join(systemRoot, "System32", "cmd.exe")
}

func replaceBinary(tempPath, execPath, backupPath string) error {
	if err := os.Chmod(tempPath, 0755); err != nil {
		return fmt.Errorf("failed to make binary executable: %w", err)
	}

	if runtime.GOOS == "windows" {
		batFile, err := os.CreateTemp("", "motd-update-*.bat")
		if err != nil {
			return fmt.Errorf("failed to create update script: %w", err)
		}
		batPath := batFile.Name()
		batFile.Close()
		defer os.Remove(batPath)

		tempBatchPath := windowsBatchPath(tempPath)
		execBatchPath := windowsBatchPath(execPath)
		backupBatchPath := windowsBatchPath(backupPath)
		batBatchPath := windowsBatchPath(batPath)
		batContent := fmt.Sprintf(`@echo off
timeout /t 1 /nobreak >nul
move /Y "%s" "%s"
if errorlevel 1 exit /b 1
del "%s"
del "%s"
`, tempBatchPath, execBatchPath, backupBatchPath, batBatchPath)

		if err := os.WriteFile(batPath, []byte(batContent), 0644); err != nil {
			return fmt.Errorf("failed to write update script: %w", err)
		}

		cmd := exec.Command(windowsCmdPath(os.Getenv("SystemRoot")), "/c", batPath)
		if err := cmd.Start(); err != nil {
			return fmt.Errorf("failed to start update script: %w", err)
		}

		fmt.Printf("%sUpdate scheduled. The binary will be replaced when this process exits.%s\n", display.Yellow, display.Reset)
		return nil
	}

	if err := os.Rename(tempPath, execPath); err != nil {
		var linkErr *os.LinkError
		if errors.As(err, &linkErr) {
			// Cross-device link: stage the binary in the target directory
			// first, then rename atomically into place.
			data, readErr := os.ReadFile(tempPath)
			if readErr != nil {
				return fmt.Errorf("failed to read downloaded binary: %w", readErr)
			}

			stageFile, stageErr := os.CreateTemp(filepath.Dir(execPath), ".motd-replace-*")
			if stageErr != nil {
				return fmt.Errorf("failed to create stage file: %w", stageErr)
			}
			stagePath := stageFile.Name()
			defer os.Remove(stagePath)

			if _, writeErr := stageFile.Write(data); writeErr != nil {
				stageFile.Close()
				return fmt.Errorf("failed to write stage file: %w", writeErr)
			}
			if syncErr := stageFile.Sync(); syncErr != nil {
				stageFile.Close()
				return fmt.Errorf("failed to sync stage file: %w", syncErr)
			}
			if chmodErr := stageFile.Chmod(0755); chmodErr != nil {
				stageFile.Close()
				return fmt.Errorf("failed to chmod stage file: %w", chmodErr)
			}
			stageFile.Close()

			if renameErr := os.Rename(stagePath, execPath); renameErr != nil {
				return fmt.Errorf("failed to rename stage file: %w", renameErr)
			}
			os.Remove(tempPath)
			return nil
		}
		return fmt.Errorf("failed to replace binary: %w", err)
	}

	return nil
}

func isInteractive() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

func windowsBatchPath(filePath string) string {
	s := filepath.ToSlash(filePath)
	s = strings.ReplaceAll(s, "/", "\\")
	s = strings.ReplaceAll(s, "%", "%%")
	s = strings.ReplaceAll(s, "^", "^^")
	s = strings.ReplaceAll(s, "&", "^&")
	s = strings.ReplaceAll(s, "|", "^|")
	s = strings.ReplaceAll(s, "<", "^<")
	s = strings.ReplaceAll(s, ">", "^>")
	s = strings.ReplaceAll(s, "(", "^(")
	s = strings.ReplaceAll(s, ")", "^)")
	s = strings.ReplaceAll(s, `"`, `""`)
	return s
}

func restoreBackup(backupPath, execPath string) error {
	if runtime.GOOS == "windows" {
		return util.CopyFile(backupPath, execPath)
	}

	if err := os.Rename(backupPath, execPath); err != nil {
		return err
	}
	return os.Chmod(execPath, 0755)
}

// checkWriteAccess verifies that the directory containing the binary is
// writable by attempting to create and remove a temporary file.
func checkWriteAccess(execPath string) error {
	dir := filepath.Dir(execPath)
	tmpFile, err := os.CreateTemp(dir, ".motd-write-test-*")
	if err != nil {
		return err
	}
	tmpFile.Close()
	return os.Remove(tmpFile.Name())
}

type cacheEntry struct {
	CheckedAt int64  `json:"checked_at"`
	Latest    string `json:"latest"`
	Message   string `json:"message"`
}

const cacheFile = "motd-version-check"
const cacheInterval = 15 * time.Minute

func fetchLatestVersionFromURL(url string, client *http.Client) (string, error) {
	resp, err := client.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxReleaseJSONSize))
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}
	if len(body) >= maxReleaseJSONSize {
		return "", fmt.Errorf("release JSON response too large (max %d bytes)", maxReleaseJSONSize)
	}

	var release GitHubRelease
	if err := json.Unmarshal(body, &release); err != nil {
		return "", fmt.Errorf("failed to parse release: %w", err)
	}

	return strings.TrimPrefix(release.TagName, "v"), nil
}

// CheckUpdate returns a non-empty update message if a newer version of motd
// is available. Results are cached for cacheInterval to avoid hammering the
// GitHub API on every motd invocation.
func CheckUpdate(currentVersion string, client *http.Client) string {
	return NewChecker().CheckUpdate(currentVersion, client)
}

func (ch *Checker) CheckUpdate(currentVersion string, client *http.Client) string {
	msg := ch.readCachedVersion()
	switch {
	case msg == "uptodate":
		return ""
	case msg != "":
		return msg
	}

	latest, err := ch.fetchLatestVersion(client)
	if err != nil {
		return ""
	}

	if CompareVersions(currentVersion, latest) >= 0 {
		ch.writeCachedVersion("uptodate")
		return ""
	}

	msg = fmt.Sprintf("An update is available for motd (%s → %s). Run 'motd self-update' to upgrade.", currentVersion, latest)
	ch.writeCachedVersion(msg)
	return msg
}

func defaultCachePath() string {
	cache, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	dir := filepath.Join(cache, "motd")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return ""
	}
	return filepath.Join(dir, cacheFile)
}

func (ch *Checker) readCachedVersion() string {
	path := ch.cachePath()
	if path == "" {
		return ""
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}

	var entry cacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return ""
	}

	if time.Since(time.Unix(entry.CheckedAt, 0)) > cacheInterval {
		return ""
	}

	return entry.Message
}

func (ch *Checker) writeCachedVersion(msg string) {
	path := ch.cachePath()
	if path == "" {
		return
	}

	entry := cacheEntry{
		CheckedAt: time.Now().Unix(),
		Message:   msg,
	}
	data, _ := json.Marshal(entry)
	_ = os.WriteFile(path, data, 0644)
}
