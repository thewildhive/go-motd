//go:build darwin

package system

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"motd/display"
	"motd/util"
)

func ShowOS(cfg ConfigAccessor, debug bool) {
	nameCmd, nameErr := util.SafeCommand("sw_vers", "-productName")
	versionCmd, versionErr := util.SafeCommand("sw_vers", "-productVersion")
	if nameErr != nil || versionErr != nil {
		return
	}
	nameOutput, nameErr := nameCmd.Output()
	versionOutput, versionErr := versionCmd.Output()
	if nameErr != nil || versionErr != nil {
		return
	}

	display.DotLabel("OS Release")
	fmt.Printf("%s%s %s%s\n", display.Blue, strings.TrimSpace(string(nameOutput)), strings.TrimSpace(string(versionOutput)), display.Reset)
}

func ShowUptime(cfg ConfigAccessor, debug bool) {
	uptime := "unknown"
	cmd, cmdErr := util.SafeCommand("sysctl", "-n", "kern.boottime")
	if cmdErr == nil {
		output, err := cmd.Output()
		if err == nil {
			if bootTime, ok := parseDarwinBootTime(output); ok {
				uptime = FormatDuration(time.Since(bootTime))
			}
		}
	}

	display.DotLabel("Uptime")
	fmt.Printf("%s%s%s\n", display.Blue, uptime, display.Reset)
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

func ShowLoad(cfg ConfigAccessor, debug bool) {
	cmd, cmdErr := util.SafeCommand("sysctl", "-n", "vm.loadavg")
	if cmdErr != nil {
		return
	}
	output, err := cmd.Output()
	if err != nil {
		return
	}

	load := strings.Trim(strings.TrimSpace(string(output)), "{}")
	display.DotLabel("CPU Load")
	fmt.Printf("%s%s%s\n", display.Blue, strings.TrimSpace(load), display.Reset)
}

func ShowMemory(cfg ConfigAccessor, debug bool) {
	totalCmd, totalErr := util.SafeCommand("sysctl", "-n", "hw.memsize")
	statsCmd, statsErr := util.SafeCommand("vm_stat")
	if totalErr != nil || statsErr != nil {
		return
	}
	totalOutput, totalErr := totalCmd.Output()
	statsOutput, statsErr := statsCmd.Output()
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

	usedGB := float64(totalBytes-freeBytes) / float64(GB)
	totalGB := float64(totalBytes) / float64(GB)
	display.DotLabel("Memory")
	fmt.Printf("%s%.2f GB / %.2f GB%s\n", display.Blue, usedGB, totalGB, display.Reset)
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

func ShowBandwidth(cfg ConfigAccessor, debug bool) {
	if !util.HasCommand("vnstat") {
		return
	}

	interfaceName := strings.TrimSpace(cfg.NetworkInterface)
	if interfaceName == "" {
		interfaceName = getDefaultInterface()
	}
	if interfaceName == "" {
		return
	}

	cmd, cmdErr := util.SafeCommand("vnstat", "--json", "m", "-i", interfaceName)
	if cmdErr != nil {
		return
	}
	output, err := cmd.Output()
	if err != nil {
		display.DebugLog(debug, "vnstat command failed: %v", err)
		return
	}

	rxGB, txGB, rxEst, txEst, err := parseVnstatMonthlyUsage(output, interfaceName, time.Now())
	if err != nil {
		display.DebugLog(debug, "Failed to parse vnstat data for %s: %v", interfaceName, err)
		return
	}

	display.DotLabel("Bandwidth (rx)")
	fmt.Printf("%s%.2f GB / %.2f GB est%s\n", display.Blue, rxGB, rxEst, display.Reset)
	display.DotLabel("Bandwidth (tx)")
	fmt.Printf("%s%.2f GB / %.2f GB est%s\n", display.Blue, txGB, txEst, display.Reset)
}

func ShowUser(cfg ConfigAccessor, debug bool) {
	cmd, cmdErr := util.SafeCommand("who")
	if cmdErr != nil {
		return
	}
	output, err := cmd.Output()
	if err != nil {
		return
	}

	count := countUniqueWhoUsers(output)
	display.DotLabel("Logged in users")
	fmt.Printf("%s%d%s\n", display.Blue, count, display.Reset)
}

func ShowProcesses(cfg ConfigAccessor, debug bool) {
	cmd, cmdErr := util.SafeCommand("ps", "-ax", "-o", "pid=")
	if cmdErr != nil {
		return
	}
	output, err := cmd.Output()
	if err != nil {
		return
	}

	count := countNonEmptyLines(output)
	display.DotLabel("Processes")
	fmt.Printf("%s%d%s\n", display.Blue, count, display.Reset)
}

func ShowDisk(cfg ConfigAccessor, debug bool) {
	showDFDisk("/", "Disk (/)")
	if cfg.TankMount != "" {
		showDFDisk(cfg.TankMount, fmt.Sprintf("Disk (%s)", cfg.TankMount))
	}
}

func showDFDisk(path, label string) {
	cmd, cmdErr := util.SafeCommand("df", "-k", path)
	if cmdErr != nil {
		return
	}
	output, err := cmd.Output()
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

	display.DotLabel(label)
	fmt.Printf("%s%.2f GB / %.2f GB (%s used)%s\n", display.Blue, usedKB/float64(MB), totalKB/float64(MB), fields[4], display.Reset)
}

func ShowTemp(cfg ConfigAccessor, debug bool) {}

func getDefaultInterface() string {
	cmd, cmdErr := util.SafeCommand("route", "-n", "get", "default")
	if cmdErr != nil {
		return ""
	}
	output, err := cmd.Output()
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
