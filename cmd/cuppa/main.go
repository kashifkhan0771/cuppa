// Command cuppa posts a daily watercooler question to team chat.
package main

import (
	"context"
	"os"
	"os/signal"

	"github.com/kashifkhan0771/cuppa/internal/cli"
)

func main() {
	// Ctrl+C cancels any request in flight instead of killing it mid-write
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	code := cli.Run(ctx, os.Args[1:], os.Stdin, os.Stdout, os.Stderr)
	stop()
	os.Exit(code)
}
