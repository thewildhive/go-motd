//go:build windows

package system

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	"motd/display"
)

func ShowOS(cfg ConfigAccessor, debug bool) {
	info, ok := getWindowsOSInfo()
	if !ok {
		info = windowsOSInfo{Version: "Unknown", Edition: "Unknown", Build: "Unknown"}
	}

	display.DotLabel("OS")
	osName := "Windows"
	if version := strings.TrimSpace(info.Version); version != "" {
		osName += " " + version
	}
	fmt.Printf("%s%s%s\n", display.Blue, osName, display.Reset)
	display.DotLabel("Edition")
	fmt.Printf("%s%s%s\n", display.Blue, valueOrUnknown(info.Edition), display.Reset)
	display.DotLabel("Build")
	fmt.Printf("%s%s%s\n", display.Blue, valueOrUnknown(info.Build), display.Reset)
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

func ShowUptime(cfg ConfigAccessor, debug bool) {
	bootTime, ok := getWindowsBootTime()
	uptime := "unknown"
	if ok {
		uptime = FormatDuration(time.Since(bootTime))
	}

	display.DotLabel("Uptime")
	fmt.Printf("%s%s%s\n", display.Blue, uptime, display.Reset)
}

func getWindowsBootTime() (time.Time, bool) {
	output, err := exec.Command("powershell", "-NoProfile", "-Command", "(Get-CimInstance Win32_OperatingSystem).LastBootUpTime").Output()
	if err == nil {
		if t, ok := parseWMICDateTime(strings.TrimSpace(string(output))); ok {
			return t, true
		}
	}

	output, err = exec.Command("wmic", "os", "get", "LastBootUpTime", "/value").Output()
	if err != nil {
		return time.Time{}, false
	}

	if output, ok := parseWMICValue(output, "LastBootUpTime"); ok {
		return parseWMICDateTime(strings.TrimSpace(output))
	}

	return time.Time{}, false
}

func ShowLoad(cfg ConfigAccessor, debug bool) {
	load, ok := getWindowsCPUPercent()
	if !ok {
		return
	}

	display.DotLabel("CPU Load")
	fmt.Printf("%s%d%%%s\n", display.Blue, load, display.Reset)
}

func getWindowsCPUPercent() (int, bool) {
	output, err := exec.Command("powershell", "-NoProfile", "-Command", "(Get-CimInstance Win32_Processor | Measure-Object -Property LoadPercentage -Average).Average").Output()
	if err == nil {
		return parseWindowsCPUPercent(output)
	}

	output, err = exec.Command("wmic", "cpu", "get", "LoadPercentage", "/value").Output()
	if err != nil {
		return 0, false
	}

	return parseWindowsCPUPercent(output)
}

func ShowMemory(cfg ConfigAccessor, debug bool) {
	total, free, ok := getWindowsMemoryBytes()
	if !ok {
		return
	}

	usedGB := float64(total-free) / 1073741824.0
	totalGB := float64(total) / 1073741824.0
	display.DotLabel("Memory")
	fmt.Printf("%s%.2f GB / %.2f GB%s\n", display.Blue, usedGB, totalGB, display.Reset)
}

func getWindowsMemoryBytes() (uint64, uint64, bool) {
	output, err := exec.Command("powershell", "-NoProfile", "-Command", "$os = Get-CimInstance Win32_OperatingSystem; '{0},{1}' -f $os.TotalVisibleMemorySize,$os.FreePhysicalMemory").Output()
	if err == nil {
		if total, free, ok := parseWindowsMemoryKB(output); ok {
			return total * 1024, free * 1024, true
		}
	}

	output, err = exec.Command("wmic", "os", "get", "TotalVisibleMemorySize,FreePhysicalMemory", "/value").Output()
	if err != nil {
		return 0, 0, false
	}

	totalStr, totalOK := parseWMICValue(output, "TotalVisibleMemorySize")
	freeStr, freeOK := parseWMICValue(output, "FreePhysicalMemory")
	if !totalOK || !freeOK {
		return 0, 0, false
	}

	totalKB, totalErr := strconv.ParseUint(totalStr, 10, 64)
	freeKB, freeErr := strconv.ParseUint(freeStr, 10, 64)
	if totalErr != nil || freeErr != nil {
		return 0, 0, false
	}

	return totalKB * 1024, freeKB * 1024, true
}

func ShowBandwidth(cfg ConfigAccessor, debug bool) {}

func ShowUser(cfg ConfigAccessor, debug bool) {}

func ShowProcesses(cfg ConfigAccessor, debug bool) {
	count, ok := getWindowsProcessCount()
	if !ok {
		return
	}

	display.DotLabel("Processes")
	fmt.Printf("%s%d%s\n", display.Blue, count, display.Reset)
}

func getWindowsProcessCount() (int, bool) {
	output, err := exec.Command("powershell", "-NoProfile", "-Command", "(Get-CimInstance Win32_Process).Count").Output()
	if err == nil {
		return countNonEmptyLines(output), true
	}

	output, err = exec.Command("tasklist", "/nh").Output()
	if err != nil {
		return 0, false
	}

	return countWindowsTasklistProcesses(output), true
}

func ShowDisk(cfg ConfigAccessor, debug bool) {
	disks, ok := getWindowsDiskInfo()
	if !ok {
		return
	}

	for _, disk := range disks {
		usedGB := float64(disk.UsedBytes) / 1073741824.0
		totalGB := float64(disk.TotalBytes) / 1073741824.0
		display.DotLabel(fmt.Sprintf("Disk (%s)", disk.Drive))
		fmt.Printf("%s%.2f GB / %.2f GB%s\n", display.Blue, usedGB, totalGB, display.Reset)
	}
}

func getWindowsDiskInfo() ([]windowsDiskInfo, bool) {
	output, err := exec.Command("powershell", "-NoProfile", "-Command", "Get-CimInstance Win32_LogicalDisk -Filter 'DriveType=3' | Select-Object DeviceID,Size,FreeSpace | Format-Csv -NoHeader").Output()
	if err == nil {
		return parseWindowsDiskCSV(output), true
	}

	output, err = exec.Command("wmic", "logicaldisk", "where", "drivetype=3", "get", "DeviceID,Size,FreeSpace", "/format:csv").Output()
	if err != nil {
		return nil, false
	}

	return parseWindowsDiskWMIC(output), true
}

func ShowTemp(cfg ConfigAccessor, debug bool) {
	if !hasCommand("powershell") {
		return
	}

	output, err := exec.Command("powershell", "-NoProfile", "-Command", "Get-CimInstance MSAcpi_ThermalZoneTemperature -Namespace 'root/wmi' | Select-Object -ExpandProperty CurrentTemperature").Output()
	if err == nil {
		if temp, ok := parseWindowsTemperature(output); ok {
			display.DotLabel("CPU Temperature")
			fmt.Printf("%s%.0f°C%s\n", display.Red, temp, display.Reset)
			return
		}
	}

	output, err = exec.Command("wmic", "/namespace:\\\\root\\wmi", "path", "MSAcpi_ThermalZoneTemperature", "get", "CurrentTemperature", "/value").Output()
	if err != nil {
		return
	}

	if temp, ok := parseWindowsTemperature(output); ok {
		display.DotLabel("CPU Temperature")
		fmt.Printf("%s%.0f°C%s\n", display.Red, temp, display.Reset)
	}
}
