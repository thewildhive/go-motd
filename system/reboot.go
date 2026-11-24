package system

import (
	"errors"
	"io/fs"
	"os"
	"runtime"

	"motd/display"
)

// RebootRequired reports whether Linux has a conventional reboot marker.
func RebootRequired(debug bool) bool {
	if runtime.GOOS != "linux" {
		return false
	}
	for _, path := range [...]string{"/run/reboot-required", "/var/run/reboot-required"} {
		info, err := os.Stat(path)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			display.DebugLog(debug, "Reboot marker unavailable: %v", err)
			return false
		}
		if !info.Mode().IsRegular() {
			display.DebugLog(debug, "Reboot marker %s is not a regular file", path)
			return false
		}
		return true
	}
	return false
}
