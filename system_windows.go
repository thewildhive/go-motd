//go:build windows

package main

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

func showOS() {
	info, ok := getWindowsOSInfo()
	if !ok {
		info = windowsOSInfo{Version: "Unknown", Edition: "Unknown", Build: "Unknown"}
	}

	dotLabel("OS")
	osName := "Windows"
	if version := strings.TrimSpace(info.Version); version != "" {
		osName += " " + version
	}
	fmt.Printf("%s%s%s\n", BLUE, osName, RESET)
	dotLabel("Edition")
	fmt.Printf("%s%s%s\n", BLUE, valueOrUnknown(info.Edition), RESET)
	dotLabel("Build")
	fmt.Printf("%s%s%s\n", BLUE, valueOrUnknown(info.Build), RESET)
}

func getWindowsOSInfo() (windowsOSInfo, bool) {
	psCommand := "$os = Get-CimInstance Win32_OperatingSystem; $cv = Get-ItemProperty 'HKLM:\\SOFTWARE\\Microsoft\\Windows NT\\CurrentVersion'; $build = [string]$os.BuildNumber; if ($null -ne $cv.UBR) { $build = '{0}.{1}' -f $os.BuildNumber,$cv.UBR }; '{0}|{1}|{2}' -f $os.Caption,$os.BuildNumber,$build"
	output, err := exec.Command("powershell", "-NoProfile", "-Command", psCommand).Output()
	if err == nil {
		if info, ok := parseWindowsOSPowerShell(output); ok {
			return info, true
		}
	}

	output, err = exec.Command("wmic", "os", "get", "Caption,BuildNumber", "/value").Output()
	if err != nil {
		return windowsOSInfo{}, false
	}

	return parseWindowsOSWMIC(output)
}

func showUptime() {
	bootTime, ok := getWindowsBootTime()
	uptime := "unknown"
	if ok {
		uptime = formatDuration(time.Since(bootTime))
	}

	dotLabel("Uptime")
	fmt.Printf("%s%s%s\n", BLUE, uptime, RESET)
}

func getWindowsBootTime() (time.Time, bool) {
	output, err := exec.Command("powershell", "-NoProfile", "-Command", "(Get-CimInstance Win32_OperatingSystem).LastBootUpTime.ToUniversalTime().ToString('o')").Output()
	if err == nil {
		bootTime, parseErr := time.Parse(time.RFC3339Nano, strings.TrimSpace(string(output)))
		if parseErr == nil {
			return bootTime, true
		}
	}

	output, err = exec.Command("wmic", "os", "get", "LastBootUpTime", "/value").Output()
	if err != nil {
		return time.Time{}, false
	}

	value, ok := parseWMICValue(output, "LastBootUpTime")
	if !ok {
		return time.Time{}, false
	}

	return parseWMICDateTime(value)
}

func showLoad() {
	load := ""
	if percent, ok := getWindowsCPUPercent(); ok {
		load = fmt.Sprintf("%d%%", percent)
	}

	dotLabel("CPU Load")
	fmt.Printf("%s%s%s\n", BLUE, load, RESET)
}

func getWindowsCPUPercent() (int, bool) {
	output, err := exec.Command("powershell", "-NoProfile", "-Command", "(Get-CimInstance Win32_Processor | Measure-Object -Property LoadPercentage -Average).Average").Output()
	if err == nil {
		value := strings.TrimSpace(string(output))
		if value != "" {
			parsed, parseErr := strconv.ParseFloat(value, 64)
			if parseErr == nil {
				return int(parsed + 0.5), true
			}
		}
	}

	output, err = exec.Command("wmic", "cpu", "get", "LoadPercentage", "/value").Output()
	if err != nil {
		return 0, false
	}

	return parseWindowsCPUPercent(output)
}

func showMemory() {
	total, free, ok := getWindowsMemoryBytes()
	if !ok || total == 0 {
		return
	}

	usedGB := float64(total-free) / 1073741824.0
	totalGB := float64(total) / 1073741824.0
	dotLabel("Memory")
	fmt.Printf("%s%.2f GB / %.2f GB%s\n", BLUE, usedGB, totalGB, RESET)
}

func getWindowsMemoryBytes() (uint64, uint64, bool) {
	output, err := exec.Command("powershell", "-NoProfile", "-Command", "$os = Get-CimInstance Win32_OperatingSystem; '{0},{1}' -f $os.TotalVisibleMemorySize,$os.FreePhysicalMemory").Output()
	if err == nil {
		if total, free, ok := parseWindowsMemoryKB(output); ok {
			return total * 1024, free * 1024, true
		}
	}

	output, err = exec.Command("wmic", "OS", "get", "FreePhysicalMemory,TotalVisibleMemorySize", "/value").Output()
	if err != nil {
		return 0, 0, false
	}

	totalKB, totalOK := parseWMICUint(output, "TotalVisibleMemorySize")
	freeKB, freeOK := parseWMICUint(output, "FreePhysicalMemory")
	if !totalOK || !freeOK {
		return 0, 0, false
	}

	return totalKB * 1024, freeKB * 1024, true
}

func showBandwidth() {}

func showUser() {}

func showProcesses() {
	count, ok := getWindowsProcessCount()
	if !ok {
		return
	}

	dotLabel("Processes")
	fmt.Printf("%s%d%s\n", BLUE, count, RESET)
}

func getWindowsProcessCount() (int, bool) {
	output, err := exec.Command("powershell", "-NoProfile", "-Command", "(Get-Process).Count").Output()
	if err == nil {
		count, parseErr := strconv.Atoi(strings.TrimSpace(string(output)))
		if parseErr == nil {
			return count, true
		}
	}

	output, err = exec.Command("tasklist", "/NH").Output()
	if err != nil {
		return 0, false
	}

	return countWindowsTasklistProcesses(output), true
}

func showDisk() {
	infos, ok := getWindowsDiskInfo()
	if !ok {
		return
	}

	for _, info := range infos {
		if info.SizeBytes == 0 {
			continue
		}

		usedBytes := info.SizeBytes - info.FreeBytes
		usedGB := float64(usedBytes) / 1073741824.0
		totalGB := float64(info.SizeBytes) / 1073741824.0
		pct := 0.0
		if info.SizeBytes > 0 {
			pct = (float64(usedBytes) / float64(info.SizeBytes)) * 100
		}

		dotLabel(fmt.Sprintf("Disk (%s)", info.Name))
		fmt.Printf("%s%.2f GB / %.2f GB (%.0f%% used)%s\n", BLUE, usedGB, totalGB, pct, RESET)
	}
}

func getWindowsDiskInfo() ([]windowsDiskInfo, bool) {
	output, err := exec.Command("powershell", "-NoProfile", "-Command", "Get-CimInstance Win32_LogicalDisk -Filter \"DriveType=3\" | ForEach-Object { '{0},{1},{2}' -f $_.DeviceID,$_.Size,$_.FreeSpace }").Output()
	if err == nil {
		if disks := parseWindowsDiskCSV(output); len(disks) > 0 {
			return disks, true
		}
	}

	output, err = exec.Command("wmic", "logicaldisk", "where", "DriveType=3", "get", "DeviceID,FreeSpace,Size", "/value").Output()
	if err != nil {
		return nil, false
	}

	disks := parseWindowsDiskWMIC(output)
	return disks, len(disks) > 0
}

func showTemp() {
	celsius, ok := getWindowsCPUTemperatureCelsius()
	if !ok {
		return
	}

	dotLabel("CPU Temperature")
	fmt.Printf("%s%.1f°C%s\n", RED, celsius, RESET)
}

func getWindowsCPUTemperatureCelsius() (float64, bool) {
	output, err := exec.Command("powershell", "-NoProfile", "-Command", "Get-CimInstance -Namespace root/wmi MSAcpi_ThermalZoneTemperature | Select-Object -First 1 -ExpandProperty CurrentTemperature").Output()
	if err != nil {
		return 0, false
	}

	return parseWindowsTemperature(output)
}
