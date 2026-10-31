// Command npm runs npm with the given arguments. It exists so `make web` works
// the same under cmd.exe, PowerShell, MSYS2 and sh: Go's exec.LookPath finds
// npm.cmd on Windows, where MSYS2's sh cannot run Node's npm shell script.
package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
)

func main() {
	cmd := exec.Command("npm", os.Args[1:]...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			os.Exit(exit.ExitCode())
		}
		fmt.Fprintln(os.Stderr, "npm:", err)
		os.Exit(1)
	}
}
