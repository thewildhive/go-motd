package util

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

// trustedUnixDirs are the trusted directories for command resolution on Linux.
var trustedUnixDirs = []string{"/usr/bin", "/usr/sbin", "/bin", "/sbin"}

// trustedMacDirs includes standard Unix paths and Homebrew paths.
var trustedMacDirs = []string{"/usr/bin", "/usr/sbin", "/bin", "/sbin", "/usr/local/bin", "/opt/homebrew/bin"}

// trustedWindowsDirs are the trusted directories on Windows.
var trustedWindowsDirs = []string{
	`C:\Windows\System32`,
	`C:\Windows\System32\WindowsPowerShell\v1.0`,
}

// platformDirs returns the trusted directories for the current platform.
func platformDirs() []string {
	switch runtime.GOOS {
	case "darwin":
		return trustedMacDirs
	case "windows":
		return trustedWindowsDirs
	default:
		return trustedUnixDirs
	}
}

// platformPath returns the platform-appropriate trusted PATH string.
func platformPath() string {
	sep := ":"
	if runtime.GOOS == "windows" {
		sep = ";"
	}
	return strings.Join(platformDirs(), sep)
}

var resolveCache sync.Map

// ResolveCommand resolves name to an absolute path by searching trusted
// system directories. Returns empty string if the command is not found.
// Results are cached to avoid redundant filesystem access.
func ResolveCommand(name string) string {
	if cached, ok := resolveCache.Load(name); ok {
		if s, ok := cached.(string); ok {
			return s
		}
		return ""
	}

	dirs := platformDirs()
	for _, dir := range dirs {
		path := filepath.Join(dir, name)
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			resolveCache.Store(name, path)
			return path
		}
	}
	// On Windows, retry with .exe extension.
	if runtime.GOOS == "windows" && !strings.HasSuffix(name, ".exe") {
		result := ResolveCommand(name + ".exe")
		resolveCache.Store(name, result)
		return result
	}
	resolveCache.Store(name, "")
	return ""
}

// HasCommand reports whether name is available in trusted directories.
func HasCommand(name string) bool {
	return ResolveCommand(name) != ""
}

// SafeCommand returns an exec.Cmd for name resolved from trusted directories
// with a minimal trusted PATH. Returns an error if name is not found.
func SafeCommand(name string, arg ...string) (*exec.Cmd, error) {
	resolved := ResolveCommand(name)
	if resolved == "" {
		return nil, fmt.Errorf("command not found in trusted directories: %s", name)
	}
	cmd := exec.Command(resolved, arg...)
	env := os.Environ()
	filtered := make([]string, 0, len(env))
	for _, e := range env {
		if !strings.HasPrefix(e, "PATH=") {
			filtered = append(filtered, e)
		}
	}
	cmd.Env = append(filtered, "PATH="+platformPath())
	return cmd, nil
}

func GetUserHome() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return home
}

func CopyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	srcInfo, err := sourceFile.Stat()
	if err != nil {
		return err
	}

	destFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, srcInfo.Mode().Perm())
	if err != nil {
		return err
	}

	if _, err := io.Copy(destFile, sourceFile); err != nil {
		destFile.Close()
		os.Remove(dst)
		return err
	}

	if err := destFile.Sync(); err != nil {
		destFile.Close()
		os.Remove(dst)
		return err
	}

	if err := destFile.Close(); err != nil {
		os.Remove(dst)
		return err
	}

	if err := os.Chmod(dst, srcInfo.Mode().Perm()); err != nil {
		os.Remove(dst)
		return err
	}

	return nil
}

func PluralSuffix(count int) string {
	if count == 1 {
		return ""
	}
	return "s"
}
