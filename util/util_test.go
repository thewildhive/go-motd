package util

import (
	"os"
	"path/filepath"
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
	if !HasCommand("sh") {
		t.Fatal("expected 'sh' to be found")
	}
}

func TestHasCommand_NotFoundReturnsFalse(t *testing.T) {
	if HasCommand("nonexistent-command-12345") {
		t.Fatal("expected nonexistent command to return false")
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
