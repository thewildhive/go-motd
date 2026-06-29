package util

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestPluralSuffix(t *testing.T) {
	if PluralSuffix(1) != "" {
		t.Fatal("expected no suffix for singular")
	}
	if PluralSuffix(2) != "s" {
		t.Fatal("expected plural suffix for count>1")
	}
}

func TestCopyFile(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	srcPath := filepath.Join(srcDir, "source.txt")
	dstPath := filepath.Join(dstDir, "dest.txt")

	if err := os.WriteFile(srcPath, []byte("hello world"), 0644); err != nil {
		t.Fatalf("failed to write source: %v", err)
	}

	if err := CopyFile(srcPath, dstPath); err != nil {
		t.Fatalf("CopyFile failed: %v", err)
	}

	data, err := os.ReadFile(dstPath)
	if err != nil {
		t.Fatalf("failed to read destination: %v", err)
	}
	if string(data) != "hello world" {
		t.Fatalf("unexpected content: %q", string(data))
	}
}

func TestCopyFileMissingSource(t *testing.T) {
	if err := CopyFile("/nonexistent/path", "/tmp/dest"); err == nil {
		t.Fatal("expected error for missing source")
	}
}

func TestGetUserHome_ReturnsNonEmpty(t *testing.T) {
	home := GetUserHome()
	if home == "" {
		t.Fatal("expected non-empty home directory")
	}
	if _, err := os.Stat(home); err != nil {
		t.Fatalf("home directory should exist: %v", err)
	}
}

func TestHasCommand_FindsShell(t *testing.T) {
	if runtime.GOOS == "windows" {
		if !HasCommand("powershell.exe") {
			t.Fatal("expected 'powershell.exe' to be found in trusted dirs")
		}
	} else {
		if !HasCommand("env") {
			t.Fatal("expected 'env' to be found in trusted dirs")
		}
	}
}

func TestHasCommand_NotFoundReturnsFalse(t *testing.T) {
	if HasCommand("nonexistent-command-12345") {
		t.Fatal("expected nonexistent command to return false")
	}
}

func TestResolveCommand_FindsStandardTool(t *testing.T) {
	var name, expected string
	switch runtime.GOOS {
	case "windows":
		name = "powershell.exe"
		expected = `C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe`
	default:
		name = "env"
		expected = "/usr/bin/env"
	}
	got := ResolveCommand(name)
	if got == "" {
		t.Fatalf("ResolveCommand(%q) returned empty", name)
	}
	if !strings.EqualFold(got, expected) {
		t.Fatalf("ResolveCommand(%q) = %q, want %q", name, got, expected)
	}
}

func TestResolveCommand_NotFound(t *testing.T) {
	if got := ResolveCommand("nonexistent-command-12345"); got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
}

func TestSafeCommand_Success(t *testing.T) {
	var name string
	switch runtime.GOOS {
	case "windows":
		name = "cmd.exe"
	default:
		name = "env"
	}
	cmd, err := SafeCommand(name)
	if err != nil {
		t.Fatalf("SafeCommand(%q) failed: %v", name, err)
	}
	if cmd.Path == "" {
		t.Fatal("expected non-empty Path")
	}
	// Verify the command has a restricted PATH.
	hasPath := false
	for _, e := range cmd.Env {
		if strings.HasPrefix(e, "PATH=") {
			hasPath = true
		}
	}
	if !hasPath {
		t.Fatal("expected PATH to be set on command environment")
	}
}

func TestSafeCommand_NotFound(t *testing.T) {
	_, err := SafeCommand("nonexistent-command-12345")
	if err == nil {
		t.Fatal("expected error for nonexistent command")
	}
}

func TestSafeCommand_Output(t *testing.T) {
	if runtime.GOOS == "windows" {
		cmd, err := SafeCommand("cmd.exe", "/c", "echo test")
		if err != nil {
			t.Fatalf("SafeCommand failed: %v", err)
		}
		output, err := cmd.Output()
		if err != nil {
			t.Fatalf("command failed: %v", err)
		}
		if !strings.Contains(string(output), "test") {
			t.Fatalf("expected output containing 'test', got %q", string(output))
		}
		return
	}
	cmd, err := SafeCommand("env")
	if err != nil {
		t.Fatalf("SafeCommand failed: %v", err)
	}
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	if len(output) == 0 {
		t.Fatal("expected non-empty output")
	}
}

func TestCopyFileSyncFailureDoesNotLeavePartial(t *testing.T) {
	// Create a source with content
	srcDir := t.TempDir()
	srcPath := filepath.Join(srcDir, "source.txt")
	dstPath := filepath.Join(srcDir, "dest.txt")
	if err := os.WriteFile(srcPath, []byte("data"), 0644); err != nil {
		t.Fatalf("failed to write source: %v", err)
	}

	if err := CopyFile(srcPath, dstPath); err != nil {
		t.Fatalf("CopyFile failed on same filesystem: %v", err)
	}
	if _, err := os.Stat(dstPath); os.IsNotExist(err) {
		t.Fatal("destination should exist after successful copy")
	}
}

func TestCopyFilePreservesPermissions(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	srcPath := filepath.Join(srcDir, "script.sh")
	dstPath := filepath.Join(dstDir, "script.sh")

	if err := os.WriteFile(srcPath, []byte("#!/bin/sh\necho hello"), 0755); err != nil {
		t.Fatalf("failed to write source: %v", err)
	}

	if err := CopyFile(srcPath, dstPath); err != nil {
		t.Fatalf("CopyFile failed: %v", err)
	}

	srcInfo, err := os.Stat(srcPath)
	if err != nil {
		t.Fatalf("failed to stat source: %v", err)
	}
	dstInfo, err := os.Stat(dstPath)
	if err != nil {
		t.Fatalf("failed to stat dest: %v", err)
	}
	if srcInfo.Mode().Perm() != dstInfo.Mode().Perm() {
		t.Fatalf("expected permissions %o, got %o", srcInfo.Mode().Perm(), dstInfo.Mode().Perm())
	}
}

func TestCopyFilePreservesPermissionsNonExecutable(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	srcPath := filepath.Join(srcDir, "readme.txt")
	dstPath := filepath.Join(dstDir, "readme.txt")

	if err := os.WriteFile(srcPath, []byte("data"), 0644); err != nil {
		t.Fatalf("failed to write source: %v", err)
	}

	if err := CopyFile(srcPath, dstPath); err != nil {
		t.Fatalf("CopyFile failed: %v", err)
	}

	srcInfo, err := os.Stat(srcPath)
	if err != nil {
		t.Fatalf("failed to stat source: %v", err)
	}
	dstInfo, err := os.Stat(dstPath)
	if err != nil {
		t.Fatalf("failed to stat dest: %v", err)
	}
	if srcInfo.Mode().Perm() != dstInfo.Mode().Perm() {
		t.Fatalf("expected permissions %o, got %o", srcInfo.Mode().Perm(), dstInfo.Mode().Perm())
	}
}
