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
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}

func PluralSuffix(count int) string {
	if count == 1 {
		return ""
	}
	return "s"
}
