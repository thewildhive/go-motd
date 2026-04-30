package main

import (
	"fmt"
	"os/exec"
	"strings"
)

func showDocker() {
	if !hasCommand("docker") {
		return
	}

	output, err := exec.Command("docker", "ps", "-q").Output()
	if err != nil {
		return
	}

	count := len(strings.Split(strings.TrimSpace(string(output)), "\n"))
	if strings.TrimSpace(string(output)) == "" {
		count = 0
	}

	dotLabel("Docker Containers")
	fmt.Printf("%s%d running%s\n", BLUE, count, RESET)
}
