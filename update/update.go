package update

import (
	"crypto/sha256"
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

	"motd/util"
)

type GitHubRelease struct {
	TagName string `json:"tag_name"`
	Name    string `json:"name"`
	Assets  []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
	} `json:"assets"`
}

func HandleSelfUpdate(version string, client *http.Client) {
	force := false
	if len(os.Args) > 2 && os.Args[2] == "--force" {
		force = true
	}

	fmt.Printf("%sChecking for updates...%s\n", "\033[0;36m", "\033[0m")

	release, err := getLatestRelease(client)
	if err != nil {
		fmt.Printf("%sError checking for updates: %v%s\n", "\033[0;31m", err, "\033[0m")
		os.Exit(1)
	}

	latestVersion := strings.TrimPrefix(release.TagName, "v")
	currentVersion := version

	fmt.Printf("Current version: %s%s%s\n", "\033[0;34m", currentVersion, "\033[0m")
	fmt.Printf("Latest version:  %s%s%s\n", "\033[0;34m", latestVersion, "\033[0m")

	if !force && CompareVersions(currentVersion, latestVersion) >= 0 {
		fmt.Printf("%sAlready running the latest version!%s\n", "\033[0;32m", "\033[0m")
		return
	}

	if !force {
		fmt.Printf("\n%sA new version is available!%s\n", "\033[0;33m", "\033[0m")
		fmt.Printf("Update from %s%s%s to %s%s%s?\n", "\033[0;31m", currentVersion, "\033[0m", "\033[0;32m", latestVersion, "\033[0m")

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
		fmt.Printf("%sError determining executable path: %v%s\n", "\033[0;31m", err, "\033[0m")
		os.Exit(1)
	}

	// Check write access to the binary directory before downloading.
	if err := checkWriteAccess(execPath); err != nil {
		localBin := filepath.Join(os.Getenv("HOME"), ".local", "bin")
		fmt.Printf("%sCannot write to %s%s\n", "\033[0;31m", filepath.Dir(execPath), "\033[0m")
		fmt.Printf("Try running with sudo, or install to %s and add it to your PATH.\n", localBin)
		fmt.Printf("  mkdir -p %s\n", localBin)
		fmt.Printf("  cp %s %s/\n", execPath, localBin)
		os.Exit(1)
	}

	fmt.Printf("\n%sUpdating to version %s...%s\n", "\033[0;36m", latestVersion, "\033[0m")

	if err := performUpdate(release, latestVersion, client); err != nil {
		fmt.Printf("%sUpdate failed: %v%s\n", "\033[0;31m", err, "\033[0m")
		os.Exit(1)
	}

	fmt.Printf("%sSuccessfully updated to version %s!%s\n", "\033[0;32m", latestVersion, "\033[0m")
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

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
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

func performUpdate(release *GitHubRelease, version string, client *http.Client) error {
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

	checksums, err := getChecksums(release, client)
	if err != nil {
		return fmt.Errorf("failed to get checksums: %w", err)
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

func getChecksums(release *GitHubRelease, client *http.Client) (map[string]string, error) {
	var checksumsURL string
	for _, asset := range release.Assets {
		if asset.Name == "checksums.txt" {
			checksumsURL = asset.URL
			break
		}
	}

	if checksumsURL == "" {
		return nil, fmt.Errorf("checksums.txt not found in release assets")
	}

	resp, err := client.Get(checksumsURL)
	if err != nil {
		return nil, fmt.Errorf("failed to download checksums: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("checksums download returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read checksums: %w", err)
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

	return checksums, nil
}

func parseChecksumLine(line string) (string, string, bool) {
	parts := strings.Fields(line)
	if len(parts) < 2 {
		return "", "", false
	}
	return parts[0], strings.TrimPrefix(parts[1], "*"), true
}

func downloadBinary(url, filename string, checksums map[string]string, tempDir string, client *http.Client) (string, error) {
	tempFile, err := os.CreateTemp(tempDir, "motd-update-*.tmp")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	defer tempFile.Close()

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

	if _, err := io.Copy(multiWriter, resp.Body); err != nil {
		return "", fmt.Errorf("failed to save binary: %w", err)
	}

	downloadedChecksum := fmt.Sprintf("%x", hasher.Sum(nil))
	expectedChecksum, exists := checksums[filename]
	if !exists {
		return "", fmt.Errorf("no checksum found for %s", filename)
	}

	if downloadedChecksum != expectedChecksum {
		return "", fmt.Errorf("checksum verification failed: expected %s, got %s", expectedChecksum, downloadedChecksum)
	}

	return tempFile.Name(), nil
}

func replaceBinary(tempPath, execPath, backupPath string) error {
	if err := os.Chmod(tempPath, 0755); err != nil {
		return fmt.Errorf("failed to make binary executable: %w", err)
	}

	if runtime.GOOS == "windows" {
		batPath := filepath.Join(os.TempDir(), "motd-update.bat")
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
			return fmt.Errorf("failed to create update script: %w", err)
		}

		cmd := exec.Command("cmd", "/c", batPath)
		if err := cmd.Start(); err != nil {
			return fmt.Errorf("failed to start update script: %w", err)
		}

		fmt.Printf("%sUpdate scheduled. The binary will be replaced when this process exits.%s\n", "\033[0;33m", "\033[0m")
		return nil
	}

	if err := os.Rename(tempPath, execPath); err != nil {
		// If rename fails with a cross-device link error (EXDEV), fall back to
		// copying the file contents and removing the source.
		var linkErr *os.LinkError
		if errors.As(err, &linkErr) {
			if copyErr := copyFileContents(tempPath, execPath); copyErr != nil {
				return fmt.Errorf("failed to replace binary (copy fallback): %w", copyErr)
			}
			if removeErr := os.Remove(tempPath); removeErr != nil {
				return fmt.Errorf("binary replaced but temp cleanup failed: %w", removeErr)
			}
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

func copyFileContents(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("failed to read source: %w", err)
	}
	if err := os.WriteFile(dst, data, 0755); err != nil {
		return fmt.Errorf("failed to write destination: %w", err)
	}
	return nil
}

func windowsBatchPath(filePath string) string {
	return strings.ReplaceAll(filepath.ToSlash(filePath), "/", "\\")
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

const cacheFile = "motd-version-check"
const cacheInterval = 15 * time.Minute

// CheckUpdate returns a non-empty update message if a newer version of motd
// is available. Results are cached for cacheInterval to avoid hammering the
// GitHub API on every motd invocation.
func CheckUpdate(currentVersion string, client *http.Client) string {
	msg := readCachedVersion()
	if msg != "" {
		return msg
	}

	latest, err := fetchLatestVersion(client)
	if err != nil {
		return ""
	}

	if CompareVersions(currentVersion, latest) >= 0 {
		writeCachedVersion("")
		return ""
	}

	msg = fmt.Sprintf("An update is available for motd (%s → %s). Run 'motd self-update' to upgrade.", currentVersion, latest)
	writeCachedVersion(msg)
	return msg
}

func cacheDir() (string, error) {
	cache, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(cache, "motd")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return dir, nil
}

func cachePath() string {
	dir, err := cacheDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, cacheFile)
}

func readCachedVersion() string {
	path := cachePath()
	if path == "" {
		return ""
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}

	parts := strings.SplitN(string(data), "\n", 2)
	if len(parts) < 2 {
		return ""
	}

	ts, err := strconv.ParseInt(strings.TrimSpace(parts[0]), 10, 64)
	if err != nil {
		return ""
	}

	if time.Since(time.Unix(ts, 0)) > cacheInterval {
		return ""
	}

	return strings.TrimSpace(parts[1])
}

func writeCachedVersion(msg string) {
	path := cachePath()
	if path == "" {
		return
	}

	data := fmt.Sprintf("%d\n%s\n", time.Now().Unix(), msg)
	_ = os.WriteFile(path, []byte(data), 0644)
}

func fetchLatestVersion(client *http.Client) (string, error) {
	url := "https://api.github.com/repos/thewildhive/go-motd/releases/latest"

	resp, err := client.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	var release GitHubRelease
	if err := json.Unmarshal(body, &release); err != nil {
		return "", fmt.Errorf("failed to parse release: %w", err)
	}

	return strings.TrimPrefix(release.TagName, "v"), nil
}
