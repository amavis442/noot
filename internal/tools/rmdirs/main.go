// Command rmdirs removes directories recursively. It exists so `make clean`
// works the same under cmd.exe, PowerShell, MSYS2 and sh.
package main

import (
	"fmt"
	"os"
)

func main() {
	failed := false
	for _, dir := range os.Args[1:] {
		if err := os.RemoveAll(dir); err != nil {
			fmt.Fprintln(os.Stderr, "rmdirs:", err)
			failed = true
		}
	}
	if failed {
		os.Exit(1)
	}
}
