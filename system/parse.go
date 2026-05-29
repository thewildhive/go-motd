package system

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
	Drive      string
	TotalBytes uint64
	UsedBytes  uint64
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
	text := strings.TrimSpace(string(output))
	parts := strings.Split(text, "|")
	if len(parts) < 3 {
		return windowsOSInfo{}, false
	}

	caption := strings.TrimSpace(parts[0])
	buildNumber := strings.TrimSpace(parts[1])
	build := strings.TrimSpace(parts[2])

	return windowsOSInfoFromCaption(caption, buildNumber, build), true
}

func parseWindowsOSWMIC(output []byte) (windowsOSInfo, bool) {
	caption, captionOK := parseWMICValue(output, "Caption")
	if !captionOK {
		return windowsOSInfo{}, false
	}

	buildNumber, _ := parseWMICValue(output, "BuildNumber")
	return windowsOSInfoFromCaption(caption, buildNumber, buildNumber), true
}

func windowsOSInfoFromCaption(caption, buildNumber, build string) windowsOSInfo {
	return windowsOSInfo{
		Version: inferWindowsVersion(caption, buildNumber),
		Edition: parseWindowsEdition(caption),
		Build:   build,
	}
}

func inferWindowsVersion(caption, buildNumber string) string {
	if buildNumber == "" {
		return ""
	}
	bn, err := strconv.Atoi(buildNumber)
	if err != nil {
		return ""
	}

	switch {
	case bn >= 22000:
		return "11"
	case bn >= 10240:
		return "10"
	default:
		return ""
	}
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
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func valueOrUnknown(value string) string {
	if value == "" {
		return "Unknown"
	}
	return value
}

func pickLatestVnstatMonth(entries []vnstatMonthlyEntry, now time.Time) (vnstatMonthlyEntry, bool) {
	if len(entries) == 0 {
		return vnstatMonthlyEntry{}, false
	}

	currentYear, currentMonth := now.Year(), int(now.Month())

	for _, entry := range entries {
		if entry.Date.Year == currentYear && entry.Date.Month == currentMonth {
			return entry, true
		}
	}

	if len(entries) >= 2 {
		return entries[len(entries)-2], true
	}

	return entries[len(entries)-1], true
}

func pickVnstatInterface(data vnstatData, preferred string) (vnstatInterface, bool) {
	if len(data.Interfaces) == 0 {
		return vnstatInterface{}, false
	}

	for _, iface := range data.Interfaces {
		if iface.ID == preferred {
			return iface, true
		}
	}

	return data.Interfaces[0], true
}

func parseVnstatMonthlyUsage(output []byte, preferredInterface string, now time.Time) (rxGB, txGB, rxEst, txEst float64, err error) {
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

	rxGB = float64(month.Rx) / 1073741824.0
	txGB = float64(month.Tx) / 1073741824.0

	day := float64(now.Day())
	if day < 1 {
		day = 1
	}

	rxEst = rxGB * (30.0 / day)
	txEst = txGB * (30.0 / day)

	return rxGB, txGB, rxEst, txEst, nil
}

func countUniqueWhoUsers(output []byte) int {
	seen := make(map[string]bool)
	for _, line := range strings.Split(string(output), "\n") {
		fields := strings.Fields(line)
		if len(fields) > 0 {
			seen[fields[0]] = true
		}
	}
	return len(seen)
}

func countNonEmptyLines(output []byte) int {
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	count := 0
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			count++
		}
	}
	return count
}

func countWindowsTasklistProcesses(output []byte) int {
	return countNonEmptyLines(output)
}

func parseWMICValue(output []byte, key string) (string, bool) {
	prefix := key + "="
	for _, line := range strings.Split(string(output), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, prefix) {
			value := strings.TrimSpace(strings.TrimPrefix(trimmed, prefix))
			if value != "" && value != key {
				return value, true
			}
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
	if err != nil {
		return 0, false
	}
	return parsed, true
}

func parseWMICDateTime(value string) (time.Time, bool) {
	// WMIC datetime format: YYYYMMDDHHMMSS.MMMMMM+OOO
	value = strings.TrimSpace(value)
	if len(value) < 14 {
		return time.Time{}, false
	}

	year, _ := strconv.Atoi(value[0:4])
	month, _ := strconv.Atoi(value[4:6])
	day, _ := strconv.Atoi(value[6:8])
	hour, _ := strconv.Atoi(value[8:10])
	min, _ := strconv.Atoi(value[10:12])
	sec, _ := strconv.Atoi(value[12:14])

	return time.Date(year, time.Month(month), day, hour, min, sec, 0, time.Local), true
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
	totalKB, totalErr := strconv.ParseUint(strings.TrimSpace(fields[0]), 10, 64)
	freeKB, freeErr := strconv.ParseUint(strings.TrimSpace(fields[1]), 10, 64)
	if totalErr != nil || freeErr != nil {
		return 0, 0, false
	}
	return totalKB, freeKB, true
}

func parseWindowsDiskCSV(output []byte) []windowsDiskInfo {
	var disks []windowsDiskInfo
	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, ",")
		if len(parts) < 3 {
			continue
		}
		drive := strings.TrimSpace(parts[0])
		totalStr := strings.TrimSpace(parts[1])
		freeStr := strings.TrimSpace(parts[2])

		totalBytes, totalErr := strconv.ParseUint(totalStr, 10, 64)
		freeBytes, freeErr := strconv.ParseUint(freeStr, 10, 64)
		if totalErr != nil || freeErr != nil || totalBytes == 0 {
			continue
		}

		disks = append(disks, windowsDiskInfo{
			Drive:      drive,
			TotalBytes: totalBytes,
			UsedBytes:  totalBytes - freeBytes,
		})
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
			disks = append(disks, windowsDiskInfo{
				Drive:      name,
				TotalBytes: size,
				UsedBytes:  size - free,
			})
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
	text := strings.TrimSpace(string(output))
	if text == "" {
		return 0, false
	}

	// Try as direct value (deci-Kelvin from wmi)
	// WMIC returns values like 3000 (30.00°C)
	// Multiple values might be returned, find the first numeric one
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		// Skip empty lines and headers
		if line == "" || strings.Contains(line, "CurrentTemperature") {
			continue
		}
		// Check for CurrentTemperature= format
		if strings.HasPrefix(line, "CurrentTemperature=") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "CurrentTemperature="))
		}
		val, err := strconv.ParseFloat(line, 64)
		if err != nil {
			continue
		}
		if val <= 0 {
			continue
		}
		// Convert from deci-Kelvin to Celsius
		celsius := (val / 10.0) - 273.15
		if celsius < 0 || celsius > 150 {
			continue
		}
		return celsius, true
	}

	return 0, false
}
