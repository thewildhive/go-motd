//go:build windows

package system

import (
	"bytes"
	"encoding/csv"
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

func valueOrUnknown(value string) string {
	if value == "" {
		return "Unknown"
	}
	return value
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
	value = strings.TrimSpace(value)
	if len(value) < 14 {
		return time.Time{}, false
	}

	year, err := strconv.Atoi(value[0:4])
	if err != nil {
		return time.Time{}, false
	}
	month, err := strconv.Atoi(value[4:6])
	if err != nil || month < 1 || month > 12 {
		return time.Time{}, false
	}
	day, err := strconv.Atoi(value[6:8])
	if err != nil || day < 1 || day > 31 {
		return time.Time{}, false
	}
	hour, err := strconv.Atoi(value[8:10])
	if err != nil || hour < 0 || hour > 23 {
		return time.Time{}, false
	}
	min, err := strconv.Atoi(value[10:12])
	if err != nil || min < 0 || min > 59 {
		return time.Time{}, false
	}
	sec, err := strconv.Atoi(value[12:14])
	if err != nil || sec < 0 || sec > 59 {
		return time.Time{}, false
	}

	return time.Date(year, time.Month(month), day, hour, min, sec, 0, time.Local), true
}

func parseWindowsCPUPercent(output []byte) (int, bool) {
	values := make([]int, 0)
	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.Contains(line, "=") {
			_, value, found := strings.Cut(line, "=")
			if !found {
				continue
			}
			line = strings.TrimSpace(value)
		}
		parsed, err := strconv.Atoi(line)
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

func parseWindowsIntOutput(output []byte) (int, bool) {
	value, err := strconv.Atoi(strings.TrimSpace(string(output)))
	if err != nil {
		return 0, false
	}
	return value, true
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

	reader := csv.NewReader(bytes.NewReader(output))
	reader.FieldsPerRecord = -1
	records, err := reader.ReadAll()
	if err != nil {
		return disks
	}

	for _, parts := range records {
		if len(parts) < 3 {
			continue
		}
		if len(parts) > 3 {
			parts = parts[len(parts)-3:]
		}
		if strings.EqualFold(strings.TrimSpace(parts[0]), "DeviceID") {
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
		name := strings.TrimSuffix(strings.TrimSpace(current["deviceid"]), "\\")
		freeValue := current["freespace"]
		sizeValue := current["size"]
		if name == "" || freeValue == "" || sizeValue == "" {
			return
		}
		free, freeErr := strconv.ParseUint(strings.TrimSpace(freeValue), 10, 64)
		size, sizeErr := strconv.ParseUint(strings.TrimSpace(sizeValue), 10, 64)
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

	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.Contains(line, "CurrentTemperature") {
			continue
		}
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
		celsius := (val / 10.0) - 273.15
		if celsius < 0 || celsius > 150 {
			continue
		}
		return celsius, true
	}

	return 0, false
}
