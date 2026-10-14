// Command datagen checks the question YAML files and compiles them
// into dist/questions.json.
//
// Usage, from the repo root:
//
//	go run ./internal/tools/datagen check
//	go run ./internal/tools/datagen build
//	go run ./internal/tools/datagen diff -base <old questions folder>
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/kashifkhan0771/cuppa/internal/questions"
)

const usage = `usage: datagen <command> [flags]

commands:
  check   validate the question files
  build   validate, then write questions.json and per-category files
  diff    print added/changed/removed questions compared to -base, as Markdown

flags:`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run does the work and returns the exit code: 0 ok, 1 failed, 2 bad usage.
func run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("datagen", flag.ContinueOnError)
	flags.SetOutput(stderr)
	questionsDir := flags.String("questions", "questions", "folder with the question YAML files")
	schemaDir := flags.String("schema", "schema", "folder with tags.txt and blocklist.txt")
	outDir := flags.String("out", "dist", "output folder (build only)")
	baseDir := flags.String("base", "", "older questions folder to compare against (diff only)")
	flags.Usage = func() {
		fmt.Fprintln(stderr, usage)
		flags.PrintDefaults()
	}

	if len(args) == 0 {
		flags.Usage()
		return 2
	}
	command := args[0]
	if err := flags.Parse(args[1:]); err != nil {
		return 2
	}

	switch command {
	case "check":
		if _, ok := loadAndCheck(*questionsDir, *schemaDir, stdout, stderr); !ok {
			return 1
		}
		return 0

	case "build":
		files, ok := loadAndCheck(*questionsDir, *schemaDir, stdout, stderr)
		if !ok {
			return 1
		}
		if err := questions.WriteDist(*outDir, questions.Compile(files)); err != nil {
			fmt.Fprintln(stderr, "error:", err)
			return 1
		}
		fmt.Fprintln(stdout, "wrote", filepath.Join(*outDir, "questions.json"))
		return 0

	case "diff":
		if *baseDir == "" {
			fmt.Fprintln(stderr, "error: diff needs -base")
			return 2
		}
		if err := printDiff(*baseDir, *questionsDir, stdout); err != nil {
			fmt.Fprintln(stderr, "error:", err)
			return 1
		}
		return 0

	default:
		fmt.Fprintf(stderr, "unknown command %q\n", command)
		flags.Usage()
		return 2
	}
}

// loadAndCheck reads the word lists and question files, prints any
// problems, and reports whether there were no errors.
func loadAndCheck(questionsDir, schemaDir string, stdout, stderr io.Writer) ([]questions.File, bool) {
	r := questions.Rules{FileNames: true}
	var err error
	if r.Tags, err = questions.ReadWordList(filepath.Join(schemaDir, "tags.txt")); err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return nil, false
	}
	if r.Blocklist, err = questions.ReadWordList(filepath.Join(schemaDir, "blocklist.txt")); err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return nil, false
	}

	files, retired, err := questions.LoadDir(questionsDir)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return nil, false
	}
	if len(files) == 0 {
		fmt.Fprintf(stderr, "error: no question files found in %s\n", questionsDir)
		return nil, false
	}

	errorCount := 0
	for _, p := range questions.Validate(files, retired, r) {
		fmt.Fprintln(stderr, p)
		if !p.Warning {
			errorCount++
		}
	}
	if errorCount > 0 {
		fmt.Fprintf(stderr, "%d errors\n", errorCount)
		return nil, false
	}

	fmt.Fprintf(stdout, "ok: %d questions in %d categories\n", len(questions.AllQuestions(files)), len(files))
	return files, true
}

// printDiff compares two question folders without validating them,
// so a broken base branch still gives a useful summary.
// A missing base folder counts as empty: every question is "added".
func printDiff(baseDir, questionsDir string, stdout io.Writer) error {
	oldFiles, _, err := questions.LoadDir(baseDir)
	if err != nil {
		return err
	}
	newFiles, _, err := questions.LoadDir(questionsDir)
	if err != nil {
		return err
	}
	return writeSummary(stdout, diffQuestions(questions.AllQuestions(oldFiles), questions.AllQuestions(newFiles)))
}
