package main

import (
	"context"
	"fmt"
	"os"

	"github.com/amavis442/noot/internal/cli"
)

func main() {
	if err := cli.NewRoot().ExecuteContext(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, "noot:", err)
		os.Exit(1)
	}
}
