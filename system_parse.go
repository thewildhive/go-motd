package main

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type windowsOSInfo struct {
	Version string
	Edition string
	Build   string
}

type windowsDiskInfo struct {
	Name      string
	SizeBytes uint64
	FreeBytes uint64
}

type vnstatMonthlyEntry struct {
	Rx   uint64 `json:"rx"`
	Tx   uint64 `json:"tx"`
	Date struct {
		Year  int `json:"year"`
		Month int `json:"month"`
	} `json:"date"`
}

type vnstatInterface struct {
	ID      string `json:"id"`
	Traffic struct {
		Month []vnstatMonthlyEntry `json:"month"`
	} `json:"traffic"`
}

type vnstatData struct {
	Interfaces []vnstatInterface `json:"interfaces"`
}

func parseWindowsOSPowerShell(output []byte) (windowsOSInfo, bool) {
	line := firstNonEmptyLine(output)
	if line == "" {
		return windowsOSInfo{}, false
	}

	parts := strings.Split(line, "|")
	if len(parts) != 3 {
		return windowsOSInfo{}, false
	}

	caption := strings.TrimSpace(parts[0])
	buildNumber := strings.TrimSpace(parts[1])
	build := strings.TrimSpace(parts[2])
	if caption == "" && build == "" {
		return windowsOSInfo{}, false
	}

	return windowsOSInfoFromCaption(caption, buildNumber, build), true
}

func parseWindowsOSWMIC(output []byte) (windowsOSInfo, bool) {
	caption, captionOK := parseWMICValue(output, "Caption")
	buildNumber, buildOK := parseWMICValue(output, "BuildNumber")
	if !captionOK && !buildOK {
		return windowsOSInfo{}, false
	}

	return windowsOSInfoFromCaption(caption, buildNumber, buildNumber), true
}

func windowsOSInfoFromCaption(caption, buildNumber, build string) windowsOSInfo {
	return windowsOSInfo{
		Version: inferWindowsVersion(caption, buildNumber),
		Edition: parseWindowsEdition(caption),
		Build:   strings.TrimSpace(build),
	}
}

func inferWindowsVersion(caption, buildNumber string) string {
	build, err := strconv.Atoi(strings.TrimSpace(buildNumber))
	if err == nil && build >= 22000 {
		return "11"
	}

	normalized := strings.ToLower(caption)
	if strings.Contains(normalized, "windows 11") {
		return "11"
	}
	if strings.Contains(normalized, "windows 10") {
		return "10"
	}
	if err == nil && build >= 10240 {
		return "10"
	}

	return ""
}

func parseWindowsEdition(caption string) string {
	fields := strings.Fields(caption)
	if len(fields) == 0 {
		return ""
	}

	start := 0
	for i, field := range fields {
		if strings.EqualFold(field, "Windows") {
			start = i + 1
			break
		}
	}

	if start < len(fields) && (fields[start] == "10" || fields[start] == "11") {
		start++
	}
	if start >= len(fields) {
		return ""
	}

	return strings.Join(fields[start:], " ")
}

func firstNonEmptyLine(output []byte) string {
	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			return line
		}
	}
	return ""
}

func valueOrUnknown(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "Unknown"
	}
	return value
}

func pickLatestVnstatMonth(entries []vnstatMonthlyEntry, now time.Time) (vnstatMonthlyEntry, bool) {
	if len(entries) == 0 {
		return vnstatMonthlyEntry{}, false
	}

	for _, entry := range entries {
		if entry.Date.Year == now.Year() && entry.Date.Month == int(now.Month()) {
			return entry, true
		}
	}

	latest := entries[0]
	for _, entry := range entries[1:] {
		if entry.Date.Year > latest.Date.Year || (entry.Date.Year == latest.Date.Year && entry.Date.Month > latest.Date.Month) {
			latest = entry
		}
	}

	return latest, true
}

func pickVnstatInterface(data vnstatData, preferred string) (vnstatInterface, bool) {
	if len(data.Interfaces) == 0 {
		return vnstatInterface{}, false
	}

	if preferred != "" {
		for _, iface := range data.Interfaces {
			if iface.ID == preferred {
				return iface, true
			}
		}
	}

	for _, iface := range data.Interfaces {
		if len(iface.Traffic.Month) > 0 {
			return iface, true
		}
	}

	return data.Interfaces[0], true
}

func parseVnstatMonthlyUsage(output []byte, preferredInterface string, now time.Time) (float64, float64, float64, float64, error) {
	var parsed vnstatData
	if err := json.Unmarshal(output, &parsed); err != nil {
		return 0, 0, 0, 0, err
	}

	iface, ok := pickVnstatInterface(parsed, preferredInterface)
	if !ok || len(iface.Traffic.Month) == 0 {
		return 0, 0, 0, 0, fmt.Errorf("no vnstat interface/monthly data available")
	}

	month, ok := pickLatestVnstatMonth(iface.Traffic.Month, now)
	if !ok {
		return 0, 0, 0, 0, fmt.Errorf("no vnstat monthly entry available")
	}

	rxGB := float64(month.Rx) / 1073741824.0
	txGB := float64(month.Tx) / 1073741824.0

	day := float64(now.Day())
	if day < 1 {
		day = 1
	}

	rxEst := rxGB * (30.0 / day)
	txEst := txGB * (30.0 / day)

	return rxGB, txGB, rxEst, txEst, nil
}

func countUniqueWhoUsers(output []byte) int {
	users := make(map[string]struct{})
	for _, line := range strings.Split(string(output), "\n") {
		fields := strings.Fields(line)
		if len(fields) > 0 {
			users[fields[0]] = struct{}{}
		}
	}
	return len(users)
}

func countNonEmptyLines(output []byte) int {
	count := 0
	for _, line := range strings.Split(string(output), "\n") {
		if strings.TrimSpace(line) != "" {
			count++
		}
	}
	return count
}

func countWindowsTasklistProcesses(output []byte) int {
	count := 0
	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "INFO:") {
			continue
		}
		count++
	}
	return count
}

func parseWMICValue(output []byte, key string) (string, bool) {
	prefix := strings.ToLower(key) + "="
	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToLower(line), prefix) {
			value := strings.TrimSpace(line[len(prefix):])
			return value, value != ""
		}
	}
	return "", false
}

func parseWMICUint(output []byte, key string) (uint64, bool) {
	value, ok := parseWMICValue(output, key)
	if !ok {
		return 0, false
	}
	parsed, err := strconv.ParseUint(value, 10, 64)
	return parsed, err == nil
}

func parseWMICDateTime(value string) (time.Time, bool) {
	if len(value) < 14 {
		return time.Time{}, false
	}

	year, errYear := strconv.Atoi(value[0:4])
	month, errMonth := strconv.Atoi(value[4:6])
	day, errDay := strconv.Atoi(value[6:8])
	hour, errHour := strconv.Atoi(value[8:10])
	minute, errMinute := strconv.Atoi(value[10:12])
	second, errSecond := strconv.Atoi(value[12:14])
	if errYear != nil || errMonth != nil || errDay != nil || errHour != nil || errMinute != nil || errSecond != nil {
		return time.Time{}, false
	}

	return time.Date(year, time.Month(month), day, hour, minute, second, 0, time.Local), true
}

func formatDuration(duration time.Duration) string {
	if duration < 0 {
		duration = 0
	}

	days := int(duration.Hours()) / 24
	hours := int(duration.Hours()) % 24
	minutes := int(duration.Minutes()) % 60
	parts := make([]string, 0, 3)
	if days > 0 {
		parts = append(parts, fmt.Sprintf("%d day%s", days, pluralSuffix(days)))
	}
	if hours > 0 {
		parts = append(parts, fmt.Sprintf("%d hour%s", hours, pluralSuffix(hours)))
	}
	if minutes > 0 || len(parts) == 0 {
		parts = append(parts, fmt.Sprintf("%d minute%s", minutes, pluralSuffix(minutes)))
	}
	return strings.Join(parts, ", ")
}

func parseWindowsCPUPercent(output []byte) (int, bool) {
	values := make([]int, 0)
	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || !strings.Contains(line, "=") {
			continue
		}
		_, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		parsed, err := strconv.Atoi(strings.TrimSpace(value))
		if err == nil {
			values = append(values, parsed)
		}
	}
	if len(values) == 0 {
		return 0, false
	}
	total := 0
	for _, value := range values {
		total += value
	}
	return total / len(values), true
}

func parseWindowsMemoryKB(output []byte) (uint64, uint64, bool) {
	fields := strings.Split(strings.TrimSpace(string(output)), ",")
	if len(fields) != 2 {
		return 0, 0, false
	}
	total, totalErr := strconv.ParseUint(strings.TrimSpace(fields[0]), 10, 64)
	free, freeErr := strconv.ParseUint(strings.TrimSpace(fields[1]), 10, 64)
	if totalErr != nil || freeErr != nil {
		return 0, 0, false
	}
	return total, free, true
}

func parseWindowsDiskCSV(output []byte) []windowsDiskInfo {
	disks := make([]windowsDiskInfo, 0)
	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, ",")
		if len(parts) != 3 {
			continue
		}
		size, sizeErr := strconv.ParseUint(strings.TrimSpace(parts[1]), 10, 64)
		free, freeErr := strconv.ParseUint(strings.TrimSpace(parts[2]), 10, 64)
		if sizeErr != nil || freeErr != nil {
			continue
		}
		disks = append(disks, windowsDiskInfo{Name: strings.TrimSpace(parts[0]), SizeBytes: size, FreeBytes: free})
	}
	return disks
}

func parseWindowsDiskWMIC(output []byte) []windowsDiskInfo {
	normalized := strings.ReplaceAll(string(output), "\r\n", "\n")
	disks := make([]windowsDiskInfo, 0)
	current := make(map[string]string)
	flush := func() {
		name := current["deviceid"]
		freeValue := current["freespace"]
		sizeValue := current["size"]
		if name == "" || freeValue == "" || sizeValue == "" {
			return
		}
		free, freeErr := strconv.ParseUint(freeValue, 10, 64)
		size, sizeErr := strconv.ParseUint(sizeValue, 10, 64)
		if freeErr == nil && sizeErr == nil {
			disks = append(disks, windowsDiskInfo{Name: name, SizeBytes: size, FreeBytes: free})
		}
		current = make(map[string]string)
	}

	for _, line := range strings.Split(normalized, "\n") {
		line = strings.TrimSpace(strings.Trim(line, "\r"))
		if line == "" {
			flush()
			continue
		}
		key, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		key = strings.ToLower(strings.TrimSpace(key))
		if key == "deviceid" && current["deviceid"] != "" {
			flush()
		}
		current[key] = strings.TrimSpace(value)
	}
	flush()
	return disks
}

func parseWindowsTemperature(output []byte) (float64, bool) {
	value := strings.TrimSpace(string(output))
	if value == "" {
		return 0, false
	}
	deciKelvin, err := strconv.ParseFloat(value, 64)
	if err != nil || deciKelvin <= 0 {
		return 0, false
	}
	return deciKelvin/10.0 - 273.15, true
}
