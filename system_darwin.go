//go:build darwin

package main

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

func showOS() {
	nameOutput, nameErr := exec.Command("sw_vers", "-productName").Output()
	versionOutput, versionErr := exec.Command("sw_vers", "-productVersion").Output()
	if nameErr != nil || versionErr != nil {
		return
	}

	dotLabel("OS Release")
	fmt.Printf("%s%s %s%s\n", BLUE, strings.TrimSpace(string(nameOutput)), strings.TrimSpace(string(versionOutput)), RESET)
}

func showUptime() {
	uptime := "unknown"
	if output, err := exec.Command("sysctl", "-n", "kern.boottime").Output(); err == nil {
		if bootTime, ok := parseDarwinBootTime(output); ok {
			uptime = formatDuration(time.Since(bootTime))
		}
	}

	dotLabel("Uptime")
	fmt.Printf("%s%s%s\n", BLUE, uptime, RESET)
}

func parseDarwinBootTime(output []byte) (time.Time, bool) {
	text := string(output)
	marker := "sec ="
	idx := strings.Index(text, marker)
	if idx < 0 {
		return time.Time{}, false
	}

	fields := strings.Fields(text[idx+len(marker):])
	if len(fields) == 0 {
		return time.Time{}, false
	}

	seconds, err := strconv.ParseInt(strings.TrimSuffix(fields[0], ","), 10, 64)
	if err != nil {
		return time.Time{}, false
	}

	return time.Unix(seconds, 0), true
}

func showLoad() {
	output, err := exec.Command("sysctl", "-n", "vm.loadavg").Output()
	if err != nil {
		return
	}

	load := strings.Trim(strings.TrimSpace(string(output)), "{}")
	dotLabel("CPU Load")
	fmt.Printf("%s%s%s\n", BLUE, strings.TrimSpace(load), RESET)
}

func showMemory() {
	totalOutput, totalErr := exec.Command("sysctl", "-n", "hw.memsize").Output()
	statsOutput, statsErr := exec.Command("vm_stat").Output()
	if totalErr != nil || statsErr != nil {
		return
	}

	totalBytes, err := strconv.ParseUint(strings.TrimSpace(string(totalOutput)), 10, 64)
	if err != nil || totalBytes == 0 {
		return
	}
	freeBytes, ok := parseDarwinFreeMemory(statsOutput)
	if !ok || freeBytes > totalBytes {
		return
	}

	usedGB := float64(totalBytes-freeBytes) / 1073741824.0
	totalGB := float64(totalBytes) / 1073741824.0
	dotLabel("Memory")
	fmt.Printf("%s%.2f GB / %.2f GB%s\n", BLUE, usedGB, totalGB, RESET)
}

func parseDarwinFreeMemory(output []byte) (uint64, bool) {
	pageSize := uint64(4096)
	freePages := uint64(0)
	speculativePages := uint64(0)

	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "page size of") {
			fields := strings.Fields(line)
			for i, field := range fields {
				if field == "of" && i+1 < len(fields) {
					if parsed, err := strconv.ParseUint(fields[i+1], 10, 64); err == nil {
						pageSize = parsed
					}
				}
			}
			continue
		}

		name, value, found := strings.Cut(line, ":")
		if !found {
			continue
		}
		pages, err := strconv.ParseUint(strings.Trim(strings.TrimSpace(value), "."), 10, 64)
		if err != nil {
			continue
		}
		switch strings.TrimSpace(name) {
		case "Pages free":
			freePages = pages
		case "Pages speculative":
			speculativePages = pages
		}
	}

	return (freePages + speculativePages) * pageSize, freePages > 0 || speculativePages > 0
}

func showBandwidth() {
	if !hasCommand("vnstat") {
		return
	}

	interfaceName := strings.TrimSpace(config.System.Network.Interface)
	if interfaceName == "" {
		interfaceName = getDefaultInterface()
	}
	if interfaceName == "" {
		return
	}

	output, err := exec.Command("vnstat", "--json", "m", "-i", interfaceName).Output()
	if err != nil {
		debugLog("vnstat command failed: %v", err)
		return
	}

	rxGB, txGB, rxEst, txEst, err := parseVnstatMonthlyUsage(output, interfaceName, time.Now())
	if err != nil {
		debugLog("Failed to parse vnstat data for %s: %v", interfaceName, err)
		return
	}

	dotLabel("Bandwidth (rx)")
	fmt.Printf("%s%.2f GB / %.2f GB est%s\n", BLUE, rxGB, rxEst, RESET)
	dotLabel("Bandwidth (tx)")
	fmt.Printf("%s%.2f GB / %.2f GB est%s\n", BLUE, txGB, txEst, RESET)
}

func showUser() {
	output, err := exec.Command("who").Output()
	if err != nil {
		return
	}

	count := countUniqueWhoUsers(output)
	dotLabel("Logged in users")
	fmt.Printf("%s%d%s\n", BLUE, count, RESET)
}

func showProcesses() {
	output, err := exec.Command("ps", "-ax", "-o", "pid=").Output()
	if err != nil {
		return
	}

	count := countNonEmptyLines(output)
	dotLabel("Processes")
	fmt.Printf("%s%d%s\n", BLUE, count, RESET)
}

func showDisk() {
	showDFDisk("/", "Disk (/)")
	if config.System.TankMount != "" {
		showDFDisk(config.System.TankMount, fmt.Sprintf("Disk (%s)", config.System.TankMount))
	}
}

func showDFDisk(path, label string) {
	output, err := exec.Command("df", "-k", path).Output()
	if err != nil {
		return
	}

	lines := strings.Split(string(output), "\n")
	if len(lines) < 2 {
		return
	}
	fields := strings.Fields(lines[1])
	if len(fields) < 5 {
		return
	}

	totalKB, totalErr := strconv.ParseFloat(fields[1], 64)
	usedKB, usedErr := strconv.ParseFloat(fields[2], 64)
	if totalErr != nil || usedErr != nil {
		return
	}

	dotLabel(label)
	fmt.Printf("%s%.2f GB / %.2f GB (%s used)%s\n", BLUE, usedKB/1048576.0, totalKB/1048576.0, fields[4], RESET)
}

func showTemp() {}

func getDefaultInterface() string {
	output, err := exec.Command("route", "-n", "get", "default").Output()
	if err != nil {
		return ""
	}

	for _, line := range strings.Split(string(output), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[0] == "interface:" {
			return fields[1]
		}
	}

	return ""
}
