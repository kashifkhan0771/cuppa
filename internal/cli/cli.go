// Package cli is the cuppa command line. Run is the whole program;
// cmd/cuppa just calls it, so every command can be tested in-process.
package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	"github.com/kashifkhan0771/cuppa/pkg/cuppa"
)

// Build info, set by the release build with -ldflags "-X ...".
var (
	Version = "dev"
	Commit  = ""
	Date    = ""
)

// Exit codes.
const (
	exitOK         = 0
	exitFailed     = 1
	exitConfig     = 2 // bad config, bad flags or bad usage
	exitPostFailed = 3 // posting failed on at least one target
	exitNoMatch    = 4 // no question matches the filter
)

// timeNow is the clock; tests set a fixed time.
var timeNow = time.Now

// codedError is an error that picks its own exit code.
type codedError struct {
	code int
	err  error
}

func (e *codedError) Error() string { return e.err.Error() }
func (e *codedError) Unwrap() error { return e.err }

func configError(err error) error {
	return &codedError{code: exitConfig, err: err}
}

type command struct {
	name    string
	summary string
	run     func(a *app, args []string) error
}

// commands is filled in init to avoid an initialization loop:
// the help command prints this list.
var commands []command

func init() {
	commands = []command{
		{"next", "print a question without posting it", runNext},
		{"post", "pick a question and post it to the targets", runPost},
		{"list", "list questions", runList},
		{"categories", "list categories", runCategories},
		{"validate", "check a config file or a question pack", runValidate},
		{"history", "show or reset the no-repeat history", runHistory},
		{"init", "create a cuppa.yaml by answering a few questions", runInit},
		{"version", "print version information", runVersion},
		{"help", "show this help", runHelp},
	}
}

// Run runs cuppa with the given arguments (without the program name)
// and returns the exit code.
func Run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	a := &app{ctx: ctx, stdin: stdin, stdout: stdout, stderr: stderr, now: timeNow}

	// global flags may come before the command; parsing stops at the
	// command name, and each command parses the rest itself
	root := flag.NewFlagSet("cuppa", flag.ContinueOnError)
	root.SetOutput(stderr)
	root.Usage = func() { printUsage(stderr) }
	a.addGlobalFlags(root)
	if err := root.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitOK
		}
		return exitConfig
	}

	if root.NArg() == 0 {
		printUsage(stderr)
		return exitConfig
	}
	name := root.Arg(0)
	for _, c := range commands {
		if c.name == name {
			return a.exitCode(c.run(a, root.Args()[1:]))
		}
	}
	fmt.Fprintf(stderr, "unknown command %q\n\n", name)
	printUsage(stderr)
	return exitConfig
}

// exitCode prints err (if any) and turns it into an exit code.
func (a *app) exitCode(err error) int {
	if err == nil {
		return exitOK
	}
	if errors.Is(err, flag.ErrHelp) {
		return exitOK
	}

	code := exitFailed
	var coded *codedError
	switch {
	case errors.As(err, &coded):
		code = coded.code
	case errors.Is(err, cuppa.ErrNoMatch):
		code = exitNoMatch
	}

	// flag errors were already printed by the flag package
	if !errors.Is(err, errFlags) {
		fmt.Fprintln(a.stderr, "error:", err)
	}
	return code
}

func runHelp(a *app, _ []string) error {
	printUsage(a.stdout)
	return nil
}

func printUsage(w io.Writer) {
	var list strings.Builder
	for _, c := range commands {
		fmt.Fprintf(&list, "  %-11s %s\n", c.name, c.summary)
	}
	fmt.Fprintf(w, `cuppa posts a watercooler question to team chat.

usage: cuppa [global flags] <command> [flags]

commands:
%s
global flags (also allowed after the command):
  --config path        config file (default: ./cuppa.yaml, then the user config folder)
  --log-level level    debug, info, warn or error (default info)
  --log-format format  text or json (default text)

run "cuppa <command> --help" to see a command's flags.
`, list.String())
}

// errFlags marks a flag parsing error that was already printed.
var errFlags = errors.New("bad flags")

// newFlagSet makes a command's flag set, with the global flags added
// so they also work after the command name.
func (a *app) newFlagSet(name, usage string) *flag.FlagSet {
	fs := flag.NewFlagSet("cuppa "+name, flag.ContinueOnError)
	fs.SetOutput(a.stderr)
	fs.Usage = func() {
		fmt.Fprintf(a.stderr, "usage: cuppa %s\n\nflags:\n", usage)
		fs.PrintDefaults()
	}
	a.addGlobalFlags(fs)
	return fs
}

// parse parses a command's flags, then sets up logging, which
// depends on the global flags.
func (a *app) parse(fs *flag.FlagSet, args []string) error {
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return err
		}
		return &codedError{code: exitConfig, err: errFlags}
	}
	return a.setupLogging()
}

func (a *app) addGlobalFlags(fs *flag.FlagSet) {
	fs.StringVar(&a.configPath, "config", a.configPath, "config file")
	fs.StringVar(&a.logLevel, "log-level", orDefault(a.logLevel, "info"), "debug, info, warn or error")
	fs.StringVar(&a.logFormat, "log-format", orDefault(a.logFormat, "text"), "text or json")
}

func (a *app) setupLogging() error {
	var level slog.Level
	if err := level.UnmarshalText([]byte(a.logLevel)); err != nil {
		return configError(fmt.Errorf("--log-level %q must be debug, info, warn or error", a.logLevel))
	}
	options := &slog.HandlerOptions{Level: level}
	switch a.logFormat {
	case "text":
		a.log = slog.New(slog.NewTextHandler(a.stderr, options))
	case "json":
		a.log = slog.New(slog.NewJSONHandler(a.stderr, options))
	default:
		return configError(fmt.Errorf("--log-format %q must be text or json", a.logFormat))
	}
	return nil
}

func orDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

// splitList turns "a, b,c" into [a b c]. Empty input gives nil.
func splitList(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}
