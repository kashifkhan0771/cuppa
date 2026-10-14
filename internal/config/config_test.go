package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// writeConfig writes content to a cuppa.yaml in a temp folder.
func writeConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "cuppa.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// the example config must always load, so it never goes stale
func TestLoadExample(t *testing.T) {
	for _, name := range []string{"GCHAT_WEBHOOK", "TEAMS_WEBHOOK", "DISCORD_WEBHOOK", "MATTERMOST_WEBHOOK", "WEBEX_TOKEN", "WEBEX_ROOM_ID", "CUPPA_TOKEN"} {
		t.Setenv(name, "x")
	}
	t.Setenv("SLACK_WEBHOOK", "https://hooks.slack.com/services/x")
	t.Setenv("MY_BOT_URL", "https://bot.example.com")
	t.Setenv("MY_BOT_TOKEN", "abc")

	cfg, warnings, err := Load(filepath.Join("..", "..", "examples", "cuppa.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 0 || len(cfg.MissingEnv) != 0 {
		t.Errorf("warnings %v, missing %v", warnings, cfg.MissingEnv)
	}
	if cfg.Selection.Mode != ModeNoRepeat || cfg.Location.String() != "Asia/Karachi" || len(cfg.Targets) != 7 {
		t.Errorf("unexpected config: %+v", cfg)
	}
	if cfg.Server.Token != "x" || cfg.Server.Addr != "127.0.0.1:8080" {
		t.Errorf("server: %+v", cfg.Server)
	}
	if cfg.Targets[5].RoomID != "x" || cfg.Targets[6].Headers["Authorization"] != "Bearer abc" {
		t.Errorf("env not filled in: %+v", cfg.Targets[5:])
	}
}

func TestLoadKeepsDefaultsForMissingKeys(t *testing.T) {
	cfg, _, err := Load(writeConfig(t, "selection:\n  mode: random\n"))
	if err != nil {
		t.Fatal(err)
	}
	want := Defaults()
	if cfg.Selection.Mode != ModeRandom || cfg.Selection.Seed != want.Selection.Seed ||
		cfg.Message.PollTemplate != DefaultPollTemplate || cfg.Version != Version {
		t.Errorf("defaults lost: %+v", cfg)
	}
	// no timezone means the computer's own, not UTC
	if cfg.Location != time.Local {
		t.Errorf("location = %v, want the local one", cfg.Location)
	}
}

func TestLoadRefresh(t *testing.T) {
	cfg, _, err := Load(writeConfig(t, "data:\n  refresh: true\n  refresh_interval: 6h\n"))
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Data.Refresh || cfg.Data.RefreshInterval != 6*time.Hour || cfg.Data.RefreshURL != DefaultRefreshURL {
		t.Errorf("got %+v", cfg.Data)
	}
}

func TestLoadEmptyFile(t *testing.T) {
	cfg, _, err := Load(writeConfig(t, ""))
	if err != nil || cfg.Selection.Mode != ModeDaily {
		t.Fatalf("got %+v, %v", cfg, err)
	}
}

func TestLoadErrors(t *testing.T) {
	tests := []struct {
		name, content, want string
	}{
		{"not yaml", "selection: [", "did not find expected"},
		{"unknown key", "selction:\n  mode: daily\n", "field selction not found"},
		{"newer version", "version: 2\n", "try upgrading cuppa"},
		{"zero version", "version: 0\n", "version must be 1"},
		{"bad mode", "selection:\n  mode: weekly\n", `selection.mode "weekly"`},
		{"bad sfw", "selection:\n  sfw: spicy\n", `selection.sfw "spicy"`},
		{"bad timezone", "schedule:\n  timezone: Mars/Base\n", `schedule.timezone "Mars/Base"`},
		{"http refresh", "data:\n  refresh: true\n  refresh_url: http://x/q.json\n", "must start with https://"},
		{"short interval", "data:\n  refresh_interval: 10s\n", "at least 1m"},
		{"bad interval", "data:\n  refresh_interval: daily\n", "into time.Duration"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := Load(writeConfig(t, tt.content))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("want error containing %q, got %v", tt.want, err)
			}
		})
	}

	if _, _, err := Load(filepath.Join(t.TempDir(), "missing.yaml")); err == nil {
		t.Error("want error for a missing file")
	}
}

func TestEnvExpansion(t *testing.T) {
	t.Setenv("HOOK", "https://hooks.example.com/a")
	t.Setenv("TRICKY", "value: with: colons\nand a newline")

	cfg, warnings, err := Load(writeConfig(t, `
selection:
  seed: "${TRICKY}"
message:
  template: "costs $5 {{ .Text }}"
targets:
  - name: a
    type: slack
    webhook: ${HOOK}
  - name: b
    type: slack
    webhook: ${NOT_SET_ANYWHERE}
`))
	if err != nil {
		t.Fatal(err)
	}

	// env values can't break the YAML, since they are filled in after parsing
	if cfg.Selection.Seed != "value: with: colons\nand a newline" {
		t.Errorf("seed = %q", cfg.Selection.Seed)
	}
	// a plain $ is left alone
	if cfg.Message.Template != "costs $5 {{ .Text }}" {
		t.Errorf("template = %q", cfg.Message.Template)
	}
	if cfg.Targets[0].Webhook != "https://hooks.example.com/a" || cfg.Targets[1].Webhook != "" {
		t.Errorf("targets = %+v", cfg.Targets)
	}
	if strings.Join(cfg.MissingEnv, ",") != "NOT_SET_ANYWHERE" {
		t.Errorf("missing = %v", cfg.MissingEnv)
	}
	if len(warnings) != 0 {
		t.Errorf("warnings = %v", warnings)
	}
}

func TestWarnsAboutPlainTextSecrets(t *testing.T) {
	_, warnings, err := Load(writeConfig(t, `
targets:
  - name: a
    type: slack
    webhook: https://hooks.slack.com/services/T0/B0/secret
`))
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "webhook is written into the config as plain text") {
		t.Errorf("warnings = %v", warnings)
	}
}

func TestPathsResolveFromConfigFolder(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home folder")
	}
	path := writeConfig(t, `
history:
  path: ~/state/history.json
data:
  extra_questions: [packs/team.yaml, /abs/pack.yaml]
`)
	cfg, _, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	dir := filepath.Dir(path)
	if cfg.History.Path != filepath.Join(home, "state", "history.json") {
		t.Errorf("history path = %s", cfg.History.Path)
	}
	if cfg.Data.ExtraQuestions[0] != filepath.Join(dir, "packs", "team.yaml") {
		t.Errorf("relative pack = %s", cfg.Data.ExtraQuestions[0])
	}
	if !filepath.IsAbs(cfg.Data.ExtraQuestions[1]) {
		t.Errorf("absolute pack changed: %s", cfg.Data.ExtraQuestions[1])
	}
}

func TestFind(t *testing.T) {
	// fake home and config folders so the real ones are never used
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "xdg"))
	t.Setenv("APPDATA", filepath.Join(home, "xdg"))
	t.Chdir(t.TempDir())

	if path, err := Find(""); path != "" || err != nil {
		t.Fatalf("nothing exists yet: got %q, %v", path, err)
	}

	// ~/.config/cuppa/cuppa.yaml is found
	dotConfig := filepath.Join(home, ".config", "cuppa", "cuppa.yaml")
	mustWrite(t, dotConfig)
	if path, _ := Find(""); path != dotConfig {
		t.Errorf("got %q, want %q", path, dotConfig)
	}

	// ./cuppa.yaml wins over the others
	mustWrite(t, "cuppa.yaml")
	if path, _ := Find(""); path != "cuppa.yaml" {
		t.Errorf("got %q, want ./cuppa.yaml", path)
	}

	// an explicit path must exist
	if _, err := Find("nope.yaml"); err == nil {
		t.Error("want error for a missing explicit path")
	}
	if path, err := Find(dotConfig); path != dotConfig || err != nil {
		t.Errorf("explicit path: got %q, %v", path, err)
	}
}

func TestDefaultHistoryPath(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", "/state")
	if got := DefaultHistoryPath(); got != filepath.Join("/state", "cuppa", "history.json") {
		t.Errorf("got %s", got)
	}

	t.Setenv("XDG_STATE_HOME", "")
	if got := DefaultHistoryPath(); !strings.HasSuffix(got, filepath.Join("cuppa", "history.json")) {
		t.Errorf("got %s", got)
	}
}

func mustWrite(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("version: 1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
}

// A version 1 file that sets every key must keep loading in every 1.x
// release. This test also fails when a key is added to the structs
// without being added to the fixture, so each new key gets a back-compat check.
func TestLoadVersion1FullFixture(t *testing.T) {
	cfg, _, err := Load(filepath.Join("testdata", "v1-full.yaml"))
	if err != nil {
		t.Fatal(err)
	}

	// every key set in the file must come out non-zero
	var empty []string
	var check func(path string, v reflect.Value)
	check = func(path string, v reflect.Value) {
		if v.Kind() == reflect.Struct {
			for i := range v.NumField() {
				field := v.Type().Field(i)
				if tag := field.Tag.Get("yaml"); tag == "-" || tag == "" {
					continue
				}
				check(path+"."+field.Name, v.Field(i))
			}
			return
		}
		if v.Kind() == reflect.Slice && v.Len() > 0 && v.Index(0).Kind() == reflect.Struct {
			check(path+"[0]", v.Index(0))
			return
		}
		if v.IsZero() {
			empty = append(empty, path)
		}
	}
	check("Config", reflect.ValueOf(*cfg))
	if len(empty) > 0 {
		t.Errorf("keys missing from testdata/v1-full.yaml: %v", empty)
	}

	// and the values must mean what they always meant
	if cfg.Schedule.Cron != "30 9 * * 1-5" || cfg.Selection.Mode != ModeNoRepeat ||
		cfg.Data.RefreshInterval != 6*time.Hour || cfg.Targets[0].Headers["X-Test"] != "1" ||
		cfg.Server.CORSOrigins[0] != "https://example.com" || cfg.Location.String() != "Asia/Karachi" {
		t.Errorf("values changed: %+v", cfg)
	}
}

// every config in examples/ must load, so the examples never go stale
func TestLoadAllExampleConfigs(t *testing.T) {
	root := filepath.Join("..", "..", "examples")
	var paths []string
	for _, pattern := range []string{"cuppa*.yaml", filepath.Join("chat-apps", "*.yaml")} {
		found, err := filepath.Glob(filepath.Join(root, pattern))
		if err != nil {
			t.Fatal(err)
		}
		paths = append(paths, found...)
	}
	if len(paths) < 10 {
		t.Fatalf("expected the example configs, found %v", paths)
	}

	for _, path := range paths {
		if _, _, err := Load(path); err != nil {
			t.Errorf("%s: %v", path, err)
		}
	}
}
