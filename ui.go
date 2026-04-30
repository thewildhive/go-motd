package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

const DOT_LABEL_WIDTH = 22

const (
	RED    = "\033[0;31m"
	GREEN  = "\033[0;32m"
	YELLOW = "\033[0;33m"
	BLUE   = "\033[0;34m"
	CYAN   = "\033[0;36m"
	BOLD   = "\033[1m"
	RESET  = "\033[0m"
)

func debugLog(msg string, args ...interface{}) {
	if debugMode {
		fmt.Fprintf(os.Stderr, "[DEBUG] "+msg+"\n", args...)
	}
}

func dotLabel(label string) {
	fmt.Print(label)
	dots := DOT_LABEL_WIDTH - len(label)
	if dots > 0 {
		fmt.Print(strings.Repeat(".", dots))
	}
	fmt.Print(": ")
}

func printHeader() {
	fmt.Println()

	hostname, _ := os.Hostname()
	if hasFiglet() && hasLolcat() {
		figlet := exec.Command("figlet", hostname)
		lolcat := exec.Command("lolcat", "-f")
		pipe, err := figlet.StdoutPipe()
		if err == nil {
			lolcat.Stdin = pipe
			lolcat.Stdout = os.Stdout
			if err := lolcat.Start(); err == nil {
				if err := figlet.Run(); err == nil {
					_ = lolcat.Wait()
				}
			}
		}
	} else {
		label := "Connected to: "
		contentLength := len(label) + len(hostname)

		totalWidth := contentLength + 5
		if totalWidth < 40 {
			totalWidth = 40
		}

		borderWidth := totalWidth - 2
		topBorder := "╔" + strings.Repeat("═", borderWidth) + "╗"
		bottomBorder := "╚" + strings.Repeat("═", borderWidth) + "╝"
		paddedContent := fmt.Sprintf("%-*s", totalWidth-5, label+hostname)

		fmt.Printf("%s%s%s%s\n", BOLD, CYAN, topBorder, RESET)
		fmt.Printf("%s%s║  %s ║%s\n", BOLD, CYAN, paddedContent, RESET)
		fmt.Printf("%s%s%s%s\n", BOLD, CYAN, bottomBorder, RESET)
	}
	fmt.Println()
}

func printSection(title string) {
	fmt.Printf("\n%s%s━━━ %s ━━━%s\n", BOLD, CYAN, title, RESET)
}

func hasFiglet() bool {
	return hasCommand("figlet")
}

func hasLolcat() bool {
	return hasCommand("lolcat")
}
