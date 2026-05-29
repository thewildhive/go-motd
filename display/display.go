package display

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

const DotLabelWidth = 22

const (
	Red    = "\033[0;31m"
	Green  = "\033[0;32m"
	Yellow = "\033[0;33m"
	Blue   = "\033[0;34m"
	Cyan   = "\033[0;36m"
	Bold   = "\033[1m"
	Reset  = "\033[0m"
)

func DebugLog(debug bool, msg string, args ...interface{}) {
	if debug {
		fmt.Fprintf(os.Stderr, "[DEBUG] "+msg+"\n", args...)
	}
}

func DotLabel(label string) {
	fmt.Print(label)
	dots := DotLabelWidth - len(label)
	if dots > 0 {
		fmt.Print(strings.Repeat(".", dots))
	}
	fmt.Print(": ")
}

// safeHostnameChars matches hostnames that are safe to pass to external commands.
// Allows letters, digits, dots, and hyphens — the POSIX-safe hostname subset.
var safeHostnameRe = regexp.MustCompile(`^[a-zA-Z0-9.-]+$`)

func PrintHeader() {
	fmt.Println()

	hostname, err := os.Hostname()
	if err != nil || hostname == "" {
		hostname = "localhost"
	}

	if hasFiglet() && hasLolcat() && safeHostnameRe.MatchString(hostname) {
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

		fmt.Printf("%s%s%s%s\n", Bold, Cyan, topBorder, Reset)
		fmt.Printf("%s%s║  %s ║%s\n", Bold, Cyan, paddedContent, Reset)
		fmt.Printf("%s%s%s%s\n", Bold, Cyan, bottomBorder, Reset)
	}
	fmt.Println()
}

func PrintSection(title string) {
	fmt.Printf("\n%s%s━━━ %s ━━━%s\n", Bold, Cyan, title, Reset)
}

func hasFiglet() bool {
	return hasCommand("figlet")
}

func hasLolcat() bool {
	return hasCommand("lolcat")
}

func hasCommand(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}
