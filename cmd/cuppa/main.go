// Command cuppa posts a daily watercooler question to team chat.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	// timezone data inside the binary, so schedule.timezone works on
	// Windows and minimal containers that have no timezone files
	_ "time/tzdata"

	"github.com/kashifkhan0771/cuppa/internal/cli"
)

func main() {
	// Ctrl+C, or SIGTERM from Docker and systemd, cancels any request
	// in flight instead of killing it mid-write, and stops the daemon
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := cli.Run(ctx, os.Args[1:], os.Stdin, os.Stdout, os.Stderr)
	stop()
	os.Exit(code)
}
