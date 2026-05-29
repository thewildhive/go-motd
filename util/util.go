package util

import (
	"io"
	"os"
	"os/exec"
)

func GetUserHome() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return home
}

func HasCommand(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func CopyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
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

	return nil
}

func PluralSuffix(count int) string {
	if count == 1 {
		return ""
	}
	return "s"
}
