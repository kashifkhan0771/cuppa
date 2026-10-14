package cli

import (
	"errors"
	"fmt"
	"os"
	"time"

	"go.yaml.in/yaml/v3"
)

// runValidate checks a config file, or a question pack when the file
// has a top-level "questions" key.
func runValidate(a *app, args []string) error {
	fs := a.newFlagSet("validate", "validate [path]")
	if err := a.parse(fs, args); err != nil {
		return err
	}
	if fs.NArg() > 1 {
		return configError(errors.New("validate takes at most one path"))
	}

	if path := fs.Arg(0); path != "" {
		isPack, err := isQuestionPack(path)
		if err != nil {
			return configError(err)
		}
		if isPack {
			return validatePack(a, path)
		}
		a.configPath = path
	}
	return validateConfig(a)
}

func validatePack(a *app, path string) error {
	dataset, problems, err := loadPack(path)
	for _, p := range problems {
		fmt.Fprintln(a.stdout, p)
	}
	if err != nil {
		return configError(err)
	}
	fmt.Fprintf(a.stdout, "ok: question pack %s has %d questions\n", path, dataset.Len())
	return nil
}

// validateConfig builds everything a post would need, without posting.
func validateConfig(a *app) error {
	if err := a.loadConfig(); err != nil {
		return err
	}
	if a.cfg.Path == "" {
		return configError(errNoConfig)
	}
	if _, err := newRenderer(a.cfg.Message); err != nil {
		return err
	}
	targets, err := a.buildTargets("")
	if err != nil {
		return err
	}
	dataset, err := a.questionsFor(a.cfg.Selection)
	if err != nil {
		return err
	}
	if dataset.Len() == 0 {
		return configError(errors.New("no questions match the selection in this config"))
	}

	// the schedule only matters to the daemon, so it is optional here
	schedule, err := a.buildSchedule()
	if err != nil {
		return err
	}
	var due time.Time
	if schedule != nil {
		if due, err = schedule.Next(a.now()); err != nil {
			return configError(fmt.Errorf("schedule: %w", err))
		}
	}

	fmt.Fprintf(a.stdout, "ok: config %s: %d questions match the selection, %d targets\n", a.cfg.Path, dataset.Len(), len(targets))
	if schedule != nil {
		fmt.Fprintf(a.stdout, "next daemon post: %s\n", due.Format("Mon 2006-01-02 15:04 MST"))
	}
	return nil
}

// isQuestionPack reports whether a YAML file looks like a question pack.
func isQuestionPack(path string) (bool, error) {
	raw, err := os.ReadFile(path) // #nosec G304 -- the user picks the file to check
	if err != nil {
		return false, err
	}
	var top map[string]any
	if err := yaml.Unmarshal(raw, &top); err != nil {
		return false, fmt.Errorf("%s: %w", path, err)
	}
	_, ok := top["questions"]
	return ok, nil
}
