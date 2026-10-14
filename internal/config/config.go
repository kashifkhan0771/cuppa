// Package config loads and checks cuppa.yaml.
//
// Compatibility rule: within config version 1, keys are only ever
// added (with defaults), never renamed or removed.
package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"
)

// Version is the config layout version this cuppa reads.
const Version = 1

// Selection modes.
const (
	ModeDaily    = "daily"
	ModeRandom   = "random"
	ModeNoRepeat = "no-repeat"
)

// Modes lists every selection mode, for checks and help text.
var Modes = []string{ModeDaily, ModeRandom, ModeNoRepeat}

// DefaultRefreshURL is where data.refresh gets questions: the newest
// release of this major version. Change the @v1 when the major changes.
const DefaultRefreshURL = "https://cdn.jsdelivr.net/gh/kashifkhan0771/cuppa@v1/dist/questions.json"

// Default message templates. Every question is posted as plain text: the
// choices of a this-or-that question are already in its text, so people
// just reply. A team that wants the options listed sets poll_template to
// one that loops over .Options (see examples/cuppa.custom-message.yaml).
// .Bold writes bold text the way each chat app expects it, and .Emojis
// has the numbers 1 to 6 for templates that number the options.
const (
	DefaultTemplate     = "☕ {{ .Bold \"Today's watercooler:\" }} {{ .Text }}"
	DefaultPollTemplate = DefaultTemplate
)

// Config is the whole cuppa.yaml file.
type Config struct {
	Version   int       `yaml:"version"`
	Schedule  Schedule  `yaml:"schedule"`
	Selection Selection `yaml:"selection"`
	Data      Data      `yaml:"data"`
	History   History   `yaml:"history"`
	Message   Message   `yaml:"message"`
	Targets   []Target  `yaml:"targets"`
	Server    Server    `yaml:"server"`

	// filled in by Load, not read from the file
	Path       string         `yaml:"-"` // file it came from; empty for defaults
	Location   *time.Location `yaml:"-"` // Schedule.Timezone, loaded
	MissingEnv []string       `yaml:"-"` // ${VARS} that were not set
}

// Schedule is when the daemon posts. Only Timezone matters to the
// other commands (it decides when the day changes in daily mode).
type Schedule struct {
	Cron         string   `yaml:"cron"`          // like "0 10 * * 1-5"; needed by the daemon
	Timezone     string   `yaml:"timezone"`      // like Asia/Karachi; empty means the computer's own
	SkipWeekends bool     `yaml:"skip_weekends"` // never post on Saturday or Sunday
	SkipDates    []string `yaml:"skip_dates"`    // never post on these days, as YYYY-MM-DD
}

type Selection struct {
	Mode        string   `yaml:"mode"`
	Seed        string   `yaml:"seed"` // daily mode: teams with different seeds get different questions
	Categories  []string `yaml:"categories"`
	Tags        []string `yaml:"tags"`
	ExcludeTags []string `yaml:"exclude_tags"`
	SFW         string   `yaml:"sfw"`
	Locale      string   `yaml:"locale"`
}

type Data struct {
	ExtraQuestions []string `yaml:"extra_questions"` // private YAML question packs

	// Refresh fetches newer questions from RefreshURL, at most once per
	// RefreshInterval, so questions stay fresh without upgrading cuppa
	Refresh         bool          `yaml:"refresh"`
	RefreshURL      string        `yaml:"refresh_url"`
	RefreshInterval time.Duration `yaml:"refresh_interval"` // like 24h
}

type History struct {
	Path string `yaml:"path"`
}

type Message struct {
	Template     string `yaml:"template"`
	PollTemplate string `yaml:"poll_template"` // for poll and this-or-that questions
}

// Target is one place to post. Which fields matter depends on Type:
// slack, gchat, teams, discord and mattermost use Webhook; webex uses
// Token and RoomID; webhook uses URL, Method, Headers, Body.
type Target struct {
	Name    string            `yaml:"name"`
	Type    string            `yaml:"type"`
	Webhook string            `yaml:"webhook"`
	URL     string            `yaml:"url"`
	Method  string            `yaml:"method"`
	Headers map[string]string `yaml:"headers"`
	Body    string            `yaml:"body"`
	Token   string            `yaml:"token"`
	RoomID  string            `yaml:"room_id"`
}

// Server configures "cuppa serve".
type Server struct {
	Addr        string   `yaml:"addr"`         // default 127.0.0.1:8080
	Token       string   `yaml:"token"`        // needed for POST /v1/post and /v1/questions/next
	CORSOrigins []string `yaml:"cors_origins"` // browser sites allowed to call the API; "*" for any
}

// Defaults returns the config used when no file exists.
func Defaults() *Config {
	return &Config{
		Version: Version,
		Selection: Selection{
			Mode:   ModeDaily,
			Seed:   "cuppa",
			SFW:    "safe",
			Locale: "en",
		},
		Data:     Data{RefreshURL: DefaultRefreshURL, RefreshInterval: 24 * time.Hour},
		History:  History{Path: DefaultHistoryPath()},
		Message:  Message{Template: DefaultTemplate, PollTemplate: DefaultPollTemplate},
		Server:   Server{Addr: "127.0.0.1:8080"},
		Location: time.Local,
	}
}

// Load reads a config file over the defaults, fills in ${VARS} from
// the environment, and checks the values. Warnings are things worth
// telling the user that don't stop cuppa from running.
func Load(path string) (cfg *Config, warnings []string, err error) {
	raw, err := os.ReadFile(path) // #nosec G304 -- the user picks their config file
	if err != nil {
		return nil, nil, err
	}

	// parse into a tree first, so ${VARS} are filled in values only;
	// an env var can then never break the YAML itself
	var tree yaml.Node
	if err := yaml.Unmarshal(raw, &tree); err != nil {
		return nil, nil, fmt.Errorf("%s: %w", path, err)
	}
	cfg = Defaults()
	env := envExpander{}
	env.walk(&tree, "")
	cfg.MissingEnv = env.missing
	warnings = env.warnings

	// decode the filled-in tree; unknown keys are an error to catch typos.
	// An empty file has no tree and just keeps the defaults.
	if tree.Kind != 0 {
		filled, err := yaml.Marshal(&tree)
		if err != nil {
			return nil, nil, fmt.Errorf("%s: %w", path, err)
		}
		dec := yaml.NewDecoder(bytes.NewReader(filled))
		dec.KnownFields(true)
		if err := dec.Decode(cfg); err != nil && !errors.Is(err, io.EOF) {
			return nil, nil, fmt.Errorf("%s: %w", path, err)
		}
	}
	cfg.Path = path

	if err := cfg.check(); err != nil {
		return nil, nil, fmt.Errorf("%s: %w", path, err)
	}
	cfg.resolvePaths()
	return cfg, warnings, nil
}

// check validates the values that don't depend on other packages.
// Targets and templates are checked when they are built.
func (c *Config) check() error {
	if c.Version > Version {
		return fmt.Errorf("config version %d is newer than this cuppa reads (%d); try upgrading cuppa", c.Version, Version)
	}
	if c.Version < 1 {
		return fmt.Errorf("version must be %d", Version)
	}
	if !slices.Contains(Modes, c.Selection.Mode) {
		return fmt.Errorf("selection.mode %q must be one of %s", c.Selection.Mode, strings.Join(Modes, ", "))
	}
	if c.Selection.SFW != "safe" && c.Selection.SFW != "casual" {
		return fmt.Errorf("selection.sfw %q must be safe or casual", c.Selection.SFW)
	}
	// https only, so nobody on the network can swap in other questions
	if c.Data.Refresh && !strings.HasPrefix(c.Data.RefreshURL, "https://") {
		return fmt.Errorf("data.refresh_url %q must start with https://", c.Data.RefreshURL)
	}
	if c.Data.RefreshInterval < time.Minute {
		return fmt.Errorf("data.refresh_interval %s must be at least 1m", c.Data.RefreshInterval)
	}

	// no timezone keeps the computer's own (from Defaults);
	// time.LoadLocation("") would give UTC instead
	if c.Schedule.Timezone != "" {
		location, err := time.LoadLocation(c.Schedule.Timezone)
		if err != nil {
			return fmt.Errorf("schedule.timezone %q: %w", c.Schedule.Timezone, err)
		}
		c.Location = location
	}
	return nil
}

// resolvePaths expands ~ and makes relative paths relative to the
// config file, so cuppa works the same from any folder.
func (c *Config) resolvePaths() {
	dir := filepath.Dir(c.Path)
	c.History.Path = resolvePath(dir, c.History.Path)
	for i, p := range c.Data.ExtraQuestions {
		c.Data.ExtraQuestions[i] = resolvePath(dir, p)
	}
}

func resolvePath(dir, path string) string {
	if path == "~" || strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			path = filepath.Join(home, strings.TrimPrefix(path, "~"))
		}
	}
	if path == "" || filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(dir, path)
}

// Find returns the config file to use: explicit if given, else the
// first that exists of ./cuppa.yaml, the user config folder, and
// ~/.config/cuppa/cuppa.yaml. It returns "" when there is none.
func Find(explicit string) (string, error) {
	if explicit != "" {
		if _, err := os.Stat(explicit); err != nil {
			return "", err
		}
		return explicit, nil
	}

	candidates := []string{"cuppa.yaml"}
	// Linux: $XDG_CONFIG_HOME or ~/.config; Windows: %APPDATA%
	if dir, err := os.UserConfigDir(); err == nil {
		candidates = append(candidates, filepath.Join(dir, "cuppa", "cuppa.yaml"))
	}
	// macOS puts UserConfigDir in Library, so also try ~/.config there
	if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates, filepath.Join(home, ".config", "cuppa", "cuppa.yaml"))
	}

	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}
	return "", nil
}

// DefaultHistoryPath is $XDG_STATE_HOME/cuppa/history.json, falling
// back to ~/.local/state, or %LocalAppData%\cuppa on Windows.
func DefaultHistoryPath() string {
	if dir := os.Getenv("XDG_STATE_HOME"); dir != "" {
		return filepath.Join(dir, "cuppa", "history.json")
	}
	if runtime.GOOS == "windows" {
		if dir, err := os.UserCacheDir(); err == nil { // %LocalAppData%
			return filepath.Join(dir, "cuppa", "history.json")
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "cuppa-history.json"
	}
	return filepath.Join(home, ".local", "state", "cuppa", "history.json")
}

// envVar matches ${NAME}. Only this form is filled in, so a plain $
// (say, in a message) is left alone.
var envVar = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)\}`)

// secretKeys are keys whose values should come from env vars.
var secretKeys = map[string]bool{"webhook": true, "token": true}

// envExpander fills ${VARS} in every string value of a YAML tree.
type envExpander struct {
	missing  []string
	warnings []string
}

func (e *envExpander) walk(node *yaml.Node, key string) {
	switch node.Kind {
	case yaml.DocumentNode, yaml.SequenceNode:
		for _, child := range node.Content {
			e.walk(child, key)
		}
	case yaml.MappingNode:
		// Content holds key, value, key, value...
		for i := 0; i+1 < len(node.Content); i += 2 {
			e.walk(node.Content[i+1], node.Content[i].Value)
		}
	case yaml.ScalarNode:
		if secretKeys[key] && node.Value != "" && !envVar.MatchString(node.Value) {
			e.warnings = append(e.warnings, fmt.Sprintf("%s is written into the config as plain text; use ${ENV_VAR} so the secret stays out of the file", key))
		}
		node.Value = envVar.ReplaceAllStringFunc(node.Value, func(match string) string {
			name := envVar.FindStringSubmatch(match)[1]
			value, ok := os.LookupEnv(name)
			if !ok && !slices.Contains(e.missing, name) {
				e.missing = append(e.missing, name)
			}
			return value
		})
	}
}
