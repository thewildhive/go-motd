package system

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

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

	rxGB = float64(month.Rx) / float64(GB)
	txGB = float64(month.Tx) / float64(GB)

	day := float64(now.Day())
	if day < 1 {
		day = 1
	}

	daysInMonth := float64(daysInMonth(now))
	rxEst = rxGB * (daysInMonth / day)
	txEst = txGB * (daysInMonth / day)

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
