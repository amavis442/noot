// Package editor builds the command that opens a file in the user's editor,
// shared by `noot open` and the terminal UI.
package editor

import (
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// Command returns the command that opens file in $VISUAL / $EDITOR (falls back
// to notepad / vi). Its standard streams are left for the caller to connect.
func Command(file string) *exec.Cmd {
	ed := os.Getenv("VISUAL")
	if ed == "" {
		ed = os.Getenv("EDITOR")
	}
	if ed == "" {
		if runtime.GOOS == "windows" {
			ed = "notepad"
		} else {
			ed = "vi"
		}
	}
	parts := strings.Fields(ed) // allows EDITOR="code -w"
	return exec.Command(parts[0], append(parts[1:], file)...)
}
