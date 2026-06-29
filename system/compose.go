package system

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"motd/display"
	"motd/util"
)

type ComposeStatus struct {
	Online int `json:"online"`
	Total  int `json:"total"`
}

type composePSLine struct {
	State  string `json:"State"`
	Status string `json:"Status"`
	Health string `json:"Health"`
}

func GetComposeStatus(cfg ConfigAccessor, debug bool) (ComposeStatus, bool) {
	composeDir := strings.TrimSpace(cfg.ComposeDir)
	if composeDir == "" || !util.HasCommand("docker") {
		return ComposeStatus{}, false
	}

	files, err := findComposeFiles(composeDir)
	if err != nil {
		display.DebugLog(debug, "Failed to inspect compose_dir %s: %v", composeDir, err)
		return ComposeStatus{}, false
	}

	status := ComposeStatus{}
	for _, file := range files {
		project, ok := getComposeFileStatus(file, debug)
		if !ok {
			continue
		}
		status.Online += project.Online
		status.Total += project.Total
	}

	return status, status.Total > 0
}

func ShowCompose(cfg ConfigAccessor, debug bool) {
	status, ok := GetComposeStatus(cfg, debug)
	if !ok {
		return
	}

	display.DotLabel("Docker Compose")
	if status.Online == status.Total {
		fmt.Printf("%sAll containers online%s\n", display.Green, display.Reset)
		return
	}
	fmt.Printf("%s%d of %d online%s\n", display.Yellow, status.Online, status.Total, display.Reset)
}

func findComposeFiles(composeDir string) ([]string, error) {
	entries, err := os.ReadDir(composeDir)
	if err != nil {
		return nil, err
	}

	files := make([]string, 0)
	if file := firstComposeFile(composeDir); file != "" {
		files = append(files, file)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if file := firstComposeFile(filepath.Join(composeDir, entry.Name())); file != "" {
			files = append(files, file)
		}
	}

	return files, nil
}

func firstComposeFile(dir string) string {
	for _, name := range []string{"compose.yml", "compose.yaml", "docker-compose.yml", "docker-compose.yaml"} {
		path := filepath.Join(dir, name)
		info, err := os.Stat(path)
		if err == nil && !info.IsDir() {
			return path
		}
	}
	return ""
}

func getComposeFileStatus(file string, debug bool) (ComposeStatus, bool) {
	cmd, err := util.SafeCommand("docker", "compose", "-f", file, "ps", "--format", "json")
	if err != nil {
		return ComposeStatus{}, false
	}
	output, err := cmd.Output()
	if err != nil {
		display.DebugLog(debug, "docker compose ps failed for %s: %v", file, err)
		return ComposeStatus{}, false
	}

	status, err := parseComposePSJSON(output)
	if err != nil {
		display.DebugLog(debug, "Failed to parse docker compose ps output for %s: %v", file, err)
		return ComposeStatus{}, false
	}
	return status, status.Total > 0
}

func parseComposePSJSON(output []byte) (ComposeStatus, error) {
	text := strings.TrimSpace(string(output))
	if text == "" {
		return ComposeStatus{}, nil
	}

	var rows []composePSLine
	if strings.HasPrefix(text, "[") {
		if err := json.Unmarshal(output, &rows); err != nil {
			return ComposeStatus{}, err
		}
	} else {
		for _, line := range strings.Split(text, "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			var row composePSLine
			if err := json.Unmarshal([]byte(line), &row); err != nil {
				return ComposeStatus{}, err
			}
			rows = append(rows, row)
		}
	}

	status := ComposeStatus{Total: len(rows)}
	for _, row := range rows {
		if composeContainerOnline(row) {
			status.Online++
		}
	}
	return status, nil
}

func composeContainerOnline(row composePSLine) bool {
	state := strings.ToLower(strings.TrimSpace(row.State))
	status := strings.ToLower(strings.TrimSpace(row.Status))
	health := strings.ToLower(strings.TrimSpace(row.Health))

	if health == "unhealthy" || strings.Contains(status, "unhealthy") {
		return false
	}
	if state == "running" || state == "up" {
		return true
	}
	return strings.HasPrefix(status, "up")
}
