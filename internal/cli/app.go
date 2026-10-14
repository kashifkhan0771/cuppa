package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/kashifkhan0771/cuppa/internal/config"
	"github.com/kashifkhan0771/cuppa/internal/questions"
	"github.com/kashifkhan0771/cuppa/pkg/cuppa"
)

// app holds what every command needs.
type app struct {
	ctx    context.Context
	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer
	now    func() time.Time // tests swap this for a fixed time

	// global flags
	configPath string
	logLevel   string
	logFormat  string

	log *slog.Logger
	cfg *config.Config
}

// loadConfig loads the config file, or the defaults when there is none.
func (a *app) loadConfig() error {
	path, err := config.Find(a.configPath)
	if err != nil {
		return configError(err)
	}
	if path == "" {
		a.log.Debug("no config file found, using defaults")
		a.cfg = config.Defaults()
		return nil
	}

	cfg, warnings, err := config.Load(path)
	if err != nil {
		return configError(err)
	}
	for _, warning := range warnings {
		a.log.Warn(warning)
	}
	a.cfg = cfg
	return nil
}

// allQuestions returns the built-in questions plus any private packs
// from the config, with nothing filtered out.
func (a *app) allQuestions() (*cuppa.Dataset, error) {
	sets := []*cuppa.Dataset{cuppa.Embedded()}
	for _, path := range a.cfg.Data.ExtraQuestions {
		pack, problems, err := loadPack(path)
		for _, p := range problems {
			a.log.Warn("question pack: " + p.String())
		}
		if err != nil {
			return nil, configError(err)
		}
		sets = append(sets, pack)
	}

	merged, overridden := cuppa.Merge(sets...)
	for _, id := range overridden {
		a.log.Warn("a question pack replaces a built-in question", "id", id)
	}
	return merged, nil
}

// loadPack reads and checks a private question pack. Packs get the
// same checks as built-in questions, except tags and blocked words:
// a team can use any tags and judge its own words.
func loadPack(path string) (*cuppa.Dataset, []questions.Problem, error) {
	file, err := questions.LoadFile(path)
	if err != nil {
		return nil, nil, err
	}

	var warnings []questions.Problem
	var errorTexts []string
	for _, p := range questions.Validate([]questions.File{file}, nil, questions.Rules{}) {
		if p.Warning {
			warnings = append(warnings, p)
		} else {
			errorTexts = append(errorTexts, p.String())
		}
	}
	if len(errorTexts) > 0 {
		return nil, warnings, fmt.Errorf("question pack %s has problems:\n  %s", path, strings.Join(errorTexts, "\n  "))
	}

	dataset, err := cuppa.New(questions.Compile([]questions.File{file}))
	return dataset, warnings, err
}

// questionsFor returns the questions that the selection allows.
func (a *app) questionsFor(s config.Selection) (*cuppa.Dataset, error) {
	all, err := a.allQuestions()
	if err != nil {
		return nil, err
	}
	return all.Filter(cuppa.Filter{
		Categories:  s.Categories,
		Tags:        s.Tags,
		ExcludeTags: s.ExcludeTags,
		SFW:         s.SFW,
		Locale:      s.Locale,
	}), nil
}

// selectionFlags are the flags that narrow down or change how a
// question is picked. next and post share them.
type selectionFlags struct {
	categories, tags, mode *string
}

func addSelectionFlags(fs *flag.FlagSet) selectionFlags {
	return selectionFlags{
		categories: fs.String("category", "", "only these categories, comma separated"),
		tags:       fs.String("tag", "", "only questions with one of these tags, comma separated"),
		mode:       fs.String("mode", "", "daily, random or no-repeat (default from config)"),
	}
}

// selectionFrom is the config's selection with these flags applied.
func (a *app) selectionFrom(f selectionFlags) (config.Selection, error) {
	return a.selection(*f.categories, *f.tags, *f.mode)
}

// selection is the config's selection with command-line overrides.
// Empty overrides keep the config value.
func (a *app) selection(categories, tags, mode string) (config.Selection, error) {
	s := a.cfg.Selection
	if list := splitList(categories); list != nil {
		s.Categories = list
	}
	if list := splitList(tags); list != nil {
		s.Tags = list
	}
	if mode != "" {
		if !slices.Contains(config.Modes, mode) {
			return s, configError(fmt.Errorf("--mode %q must be one of %s", mode, strings.Join(config.Modes, ", ")))
		}
		s.Mode = mode
	}
	return s, nil
}

// pick chooses a question the way the mode says. Only no-repeat
// uses (and changes) the history.
func (a *app) pick(d *cuppa.Dataset, s config.Selection, h cuppa.History) (cuppa.Question, error) {
	switch s.Mode {
	case config.ModeRandom:
		return cuppa.Random(d, nil)
	case config.ModeNoRepeat:
		return cuppa.Next(d, h, nil)
	default:
		// the date is the team's date, from the config's time zone
		return cuppa.Daily(d, s.Seed, a.now().In(a.cfg.Location))
	}
}

// missingEnvHint explains an empty value that came from unset env vars.
func (a *app) missingEnvHint() string {
	if len(a.cfg.MissingEnv) == 0 {
		return ""
	}
	return " (these environment variables are not set: " + strings.Join(a.cfg.MissingEnv, ", ") + ")"
}

// errNoConfig is returned by commands that need a config file.
var errNoConfig = errors.New(`no config file found; create one with "cuppa init" or pass --config`)
