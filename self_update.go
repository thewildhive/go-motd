package main

import (
	"crypto/sha256"
	"encoding/json"
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
)

type GitHubRelease struct {
	TagName string `json:"tag_name"`
	Name    string `json:"name"`
	Assets  []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
	} `json:"assets"`
}

func handleSelfUpdate() {
	force := false
	if len(os.Args) > 2 && os.Args[2] == "--force" {
		force = true
	}

	httpClient = &http.Client{
		Timeout: CURL_TIMEOUT,
		Transport: &http.Transport{
			MaxIdleConns:       10,
			IdleConnTimeout:    30 * time.Second,
			DisableCompression: false,
		},
	}

	fmt.Printf("%sChecking for updates...%s\n", CYAN, RESET)

	release, err := getLatestRelease()
	if err != nil {
		fmt.Printf("%sError checking for updates: %v%s\n", RED, err, RESET)
		os.Exit(1)
	}

	latestVersion := strings.TrimPrefix(release.TagName, "v")
	currentVersion := VERSION

	fmt.Printf("Current version: %s%s%s\n", BLUE, currentVersion, RESET)
	fmt.Printf("Latest version:  %s%s%s\n", BLUE, latestVersion, RESET)

	if !force && compareVersions(currentVersion, latestVersion) >= 0 {
		fmt.Printf("%sAlready running the latest version!%s\n", GREEN, RESET)
		return
	}

	if !force {
		fmt.Printf("\n%sA new version is available!%s\n", YELLOW, RESET)
		fmt.Printf("Update from %s%s%s to %s%s%s?\n", RED, currentVersion, RESET, GREEN, latestVersion, RESET)
		fmt.Print("Update? [y/N]: ")

		var response string
		fmt.Scanln(&response)
		if strings.ToLower(response) != "y" && strings.ToLower(response) != "yes" {
			fmt.Println("Update cancelled.")
			return
		}
	}

	fmt.Printf("\n%sUpdating to version %s...%s\n", CYAN, latestVersion, RESET)

	if err := performUpdate(release, latestVersion); err != nil {
		fmt.Printf("%sUpdate failed: %v%s\n", RED, err, RESET)
		os.Exit(1)
	}

	fmt.Printf("%sSuccessfully updated to version %s!%s\n", GREEN, latestVersion, RESET)
}

func getLatestRelease() (*GitHubRelease, error) {
	url := "https://api.github.com/repos/thewildhive/go-motd/releases/latest"

	resp, err := httpClient.Get(url)
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

func compareVersions(current, latest string) int {
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

func performUpdate(release *GitHubRelease, version string) error {
	assetName := getPlatformAssetName()
	if assetName == "" {
		return fmt.Errorf("unsupported platform: %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	var downloadURL string
	for _, asset := range release.Assets {
		if strings.Contains(asset.Name, assetName) {
			downloadURL = asset.URL
			break
		}
	}

	if downloadURL == "" {
		return fmt.Errorf("could not find release asset for platform %s", assetName)
	}

	checksums, err := getChecksums(release)
	if err != nil {
		return fmt.Errorf("failed to get checksums: %w", err)
	}

	tempFile, err := downloadBinary(downloadURL, assetName, checksums)
	if err != nil {
		return fmt.Errorf("failed to download binary: %w", err)
	}
	removeTemp := true
	defer func() {
		if removeTemp {
			_ = os.Remove(tempFile)
		}
	}()

	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	backupPath := execPath + ".backup"
	if err := copyFile(execPath, backupPath); err != nil {
		return fmt.Errorf("failed to create backup: %w", err)
	}
	defer os.Remove(backupPath)

	if err := replaceBinary(tempFile, execPath); err != nil {
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

func getChecksums(release *GitHubRelease) (map[string]string, error) {
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

	resp, err := httpClient.Get(checksumsURL)
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
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			checksums[parts[1]] = parts[0]
		}
	}

	return checksums, nil
}

func downloadBinary(url, filename string, checksums map[string]string) (string, error) {
	tempFile, err := os.CreateTemp("", "motd-update-*.tmp")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	defer tempFile.Close()

	resp, err := httpClient.Get(url)
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

func replaceBinary(tempPath, execPath string) error {
	if err := os.Chmod(tempPath, 0755); err != nil {
		return fmt.Errorf("failed to make binary executable: %w", err)
	}

	if runtime.GOOS == "windows" {
		batPath := filepath.Join(os.TempDir(), "motd-update.bat")
		batContent := fmt.Sprintf(`@echo off
timeout /t 1 /nobreak >nul
move /Y "%s" "%s"
del "%s"
`, tempPath, execPath, batPath)

		if err := os.WriteFile(batPath, []byte(batContent), 0644); err != nil {
			return fmt.Errorf("failed to create update script: %w", err)
		}

		cmd := exec.Command("cmd", "/c", batPath)
		if err := cmd.Start(); err != nil {
			return fmt.Errorf("failed to start update script: %w", err)
		}

		fmt.Printf("%sUpdate scheduled. The binary will be replaced when this process exits.%s\n", YELLOW, RESET)
		return nil
	}

	if err := os.Rename(tempPath, execPath); err != nil {
		return fmt.Errorf("failed to replace binary: %w", err)
	}

	return nil
}

func restoreBackup(backupPath, execPath string) error {
	if runtime.GOOS == "windows" {
		return copyFile(backupPath, execPath)
	}

	if err := os.Rename(backupPath, execPath); err != nil {
		return err
	}
	return os.Chmod(execPath, 0755)
}
