package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kashifkhan0771/cuppa/internal/config"
	"github.com/kashifkhan0771/cuppa/internal/history"
	"github.com/kashifkhan0771/cuppa/pkg/cuppa"
)

var fixedNow = time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC)

func TestMain(m *testing.M) {
	timeNow = func() time.Time { return fixedNow }
	os.Exit(m.Run())
}

type result struct {
	code           int
	stdout, stderr string
}

// run runs cuppa in-process with the given stdin and arguments.
func run(t *testing.T, stdin string, args ...string) result {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), args, strings.NewReader(stdin), &stdout, &stderr)
	return result{code: code, stdout: stdout.String(), stderr: stderr.String()}
}

// isolate points home, config and state folders at a temp dir and
// moves into it, so tests never see or change real user files.
func isolate(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, name := range []string{"HOME", "USERPROFILE", "XDG_CONFIG_HOME", "XDG_STATE_HOME", "XDG_CACHE_HOME", "APPDATA", "LOCALAPPDATA"} {
		t.Setenv(name, dir)
	}
	t.Chdir(dir)
	return dir
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

// chat is a fake chat service that records what it receives.
type chat struct {
	mu     sync.Mutex
	status int
	bodies []string
	url    string
}

func newChat(t *testing.T, status int) *chat {
	t.Helper()
	c := &chat{status: status}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		c.mu.Lock()
		c.bodies = append(c.bodies, string(raw))
		c.mu.Unlock()
		w.WriteHeader(c.status)
	}))
	t.Cleanup(srv.Close)
	c.url = srv.URL
	return c
}

func (c *chat) received() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), c.bodies...)
}

func TestHelpAndUsage(t *testing.T) {
	isolate(t)

	if r := run(t, "", "help"); r.code != exitOK || !strings.Contains(r.stdout, "commands:") {
		t.Errorf("help: %+v", r)
	}
	if r := run(t, ""); r.code != exitConfig || !strings.Contains(r.stderr, "usage: cuppa") {
		t.Errorf("no args: %+v", r)
	}
	if r := run(t, "", "--help"); r.code != exitOK {
		t.Errorf("--help: %+v", r)
	}
	if r := run(t, "", "next", "--help"); r.code != exitOK || !strings.Contains(r.stderr, "usage: cuppa next") {
		t.Errorf("next --help: %+v", r)
	}
	if r := run(t, "", "bogus"); r.code != exitConfig || !strings.Contains(r.stderr, `unknown command "bogus"`) {
		t.Errorf("unknown command: %+v", r)
	}
	if r := run(t, "", "next", "--nope"); r.code != exitConfig || strings.Contains(r.stderr, "error: bad flags") {
		t.Errorf("bad flag: %+v", r)
	}
	if r := run(t, "", "--nope"); r.code != exitConfig {
		t.Errorf("bad global flag: %+v", r)
	}
}

func TestLoggingFlags(t *testing.T) {
	isolate(t)
	if r := run(t, "", "next", "--log-level", "loud"); r.code != exitConfig || !strings.Contains(r.stderr, "--log-level") {
		t.Errorf("bad level: %+v", r)
	}
	if r := run(t, "", "--log-format", "xml", "next"); r.code != exitConfig || !strings.Contains(r.stderr, "--log-format") {
		t.Errorf("bad format: %+v", r)
	}
	if r := run(t, "", "next", "--log-format", "json", "--log-level", "debug"); r.code != exitOK || !strings.Contains(r.stderr, `"msg":"no config file found`) {
		t.Errorf("json debug log: %+v", r)
	}
}

func TestNextWithoutConfig(t *testing.T) {
	isolate(t)

	// default mode is daily: same answer as the library gives for today
	want, err := cuppa.Daily(cuppa.Embedded().Filter(cuppa.Filter{SFW: "safe", Locale: "en"}), "cuppa", fixedNow.In(time.Local))
	if err != nil {
		t.Fatal(err)
	}
	r := run(t, "", "next")
	if r.code != exitOK || !strings.Contains(r.stdout, want.Text) || !strings.HasPrefix(r.stdout, "☕") {
		t.Fatalf("got %+v, want text %q", r, want.Text)
	}

	r = run(t, "", "next", "--json", "--mode", "random", "--category", "this-or-that")
	var q cuppa.Question
	if err := json.Unmarshal([]byte(r.stdout), &q); err != nil || q.Category != "this-or-that" {
		t.Fatalf("json: %+v %v", r, err)
	}
}

// by default only the question is posted: the choices are in its text
func TestNextPostsOnlyTheQuestion(t *testing.T) {
	isolate(t)
	r := run(t, "", "next", "--category", "this-or-that")
	if r.code != exitOK || !strings.Contains(r.stdout, "☕") || strings.Contains(r.stdout, "1️⃣") || strings.Count(strings.TrimSpace(r.stdout), "\n") != 0 {
		t.Fatalf("got %+v", r)
	}
}

// a team can still ask for the options to be listed
func TestNextListsOptionsWithPollTemplate(t *testing.T) {
	isolate(t)
	writeFile(t, "cuppa.yaml", "message:\n  poll_template: '{{ .Text }}{{ range $i, $o := .Options }} {{ index $.Emojis $i }} {{ $o }}{{ end }}'\n")
	r := run(t, "", "next", "--category", "this-or-that")
	if r.code != exitOK || !strings.Contains(r.stdout, "1️⃣ ") || !strings.Contains(r.stdout, "2️⃣ ") {
		t.Fatalf("got %+v", r)
	}
}

func TestNextErrors(t *testing.T) {
	isolate(t)
	if r := run(t, "", "next", "--mode", "weekly"); r.code != exitConfig {
		t.Errorf("bad mode: %+v", r)
	}
	if r := run(t, "", "next", "--category", "nope"); r.code != exitNoMatch {
		t.Errorf("no match: %+v", r)
	}
	if r := run(t, "", "next", "--config", "missing.yaml"); r.code != exitConfig {
		t.Errorf("missing config: %+v", r)
	}
}

func TestNextNoRepeatMarksHistory(t *testing.T) {
	dir := isolate(t)
	writeFile(t, "cuppa.yaml", "selection:\n  mode: no-repeat\nhistory:\n  path: state/history.json\n")

	if r := run(t, "", "next", "--json"); r.code != exitOK {
		t.Fatalf("got %+v", r)
	}
	used, err := history.NewFile(filepath.Join(dir, "state", "history.json")).Used()
	if err != nil || len(used) != 1 {
		t.Fatalf("used = %v, %v", used, err)
	}
}

func TestListAndCategories(t *testing.T) {
	isolate(t)

	r := run(t, "", "list", "--category", "icebreakers")
	if r.code != exitOK || !strings.Contains(r.stdout, "ice-0001") || strings.Contains(r.stdout, "wyr-") {
		t.Errorf("list: %+v", r)
	}

	r = run(t, "", "list", "--json")
	var all []cuppa.Question
	if err := json.Unmarshal([]byte(r.stdout), &all); err != nil || len(all) == 0 {
		t.Errorf("list --json: %v", err)
	}

	r = run(t, "", "categories")
	if r.code != exitOK || !strings.Contains(r.stdout, "would-you-rather") || !strings.Contains(r.stdout, "30 questions") {
		t.Errorf("categories: %+v", r)
	}

	r = run(t, "", "categories", "--json")
	var categories []cuppa.Category
	if err := json.Unmarshal([]byte(r.stdout), &categories); err != nil || len(categories) != 5 {
		t.Errorf("categories --json: %v %v", categories, err)
	}
}

func TestVersion(t *testing.T) {
	isolate(t)
	r := run(t, "", "version")
	if r.code != exitOK || !strings.Contains(r.stdout, "cuppa dev") || !strings.Contains(r.stdout, cuppa.Embedded().Version()) {
		t.Errorf("got %+v", r)
	}

	Commit, Date = "abc123", "2026-10-14"
	defer func() { Commit, Date = "", "" }()
	if r := run(t, "", "version"); !strings.Contains(r.stdout, "commit abc123") {
		t.Errorf("got %+v", r)
	}
}

func TestHistoryCommand(t *testing.T) {
	isolate(t)
	writeFile(t, "cuppa.yaml", "selection:\n  mode: no-repeat\nhistory:\n  path: h.json\n")
	run(t, "", "next")

	if r := run(t, "", "history"); r.code != exitOK || !strings.Contains(r.stdout, "used this cycle: 1 of") {
		t.Errorf("show: %+v", r)
	}
	if r := run(t, "", "history", "reset"); r.code != exitOK {
		t.Errorf("reset: %+v", r)
	}
	if r := run(t, "", "history", "show"); !strings.Contains(r.stdout, "used this cycle: 0 of") || !strings.Contains(r.stdout, "finished cycles: 1") {
		t.Errorf("after reset: %+v", r)
	}
	if r := run(t, "", "history", "wipe"); r.code != exitConfig {
		t.Errorf("bad action: %+v", r)
	}
}

func TestInit(t *testing.T) {
	isolate(t)

	r := run(t, "gchat\nTEAM_HOOK\nno-repeat\n", "init")
	if r.code != exitOK || !strings.Contains(r.stdout, "export TEAM_HOOK=") {
		t.Fatalf("init: %+v", r)
	}

	// the written file must load and point at the env var
	t.Setenv("TEAM_HOOK", "https://chat.googleapis.com/v1/spaces/x")
	if r := run(t, "", "validate"); r.code != exitOK {
		t.Fatalf("validate after init: %+v", r)
	}

	if r := run(t, "", "init"); r.code != exitConfig || !strings.Contains(r.stderr, "already exists") {
		t.Errorf("overwrite: %+v", r)
	}
	// empty answers (or no input at all) take the defaults
	if r := run(t, "", "init", "--force"); r.code != exitOK {
		t.Errorf("--force: %+v", r)
	}
	content, _ := os.ReadFile("cuppa.yaml")
	if !strings.Contains(string(content), "type: slack") || !strings.Contains(string(content), "${SLACK_WEBHOOK}") {
		t.Errorf("defaults not used:\n%s", content)
	}

	if r := run(t, "webhook\n\n\n", "init", "--output", "hook.yaml"); r.code != exitOK {
		t.Errorf("webhook init: %+v", r)
	}
	content, _ = os.ReadFile("hook.yaml")
	if !strings.Contains(string(content), "url: ${WEBHOOK_WEBHOOK}") {
		t.Errorf("webhook config:\n%s", content)
	}

	// webex asks for two env vars: the bot token and the room ID
	if r := run(t, "webex\n\n\n\n", "init", "--output", "webex.yaml"); r.code != exitOK || !strings.Contains(r.stdout, "export WEBEX_ROOM_ID=") {
		t.Errorf("webex init: %+v", r)
	}
	content, _ = os.ReadFile("webex.yaml")
	if !strings.Contains(string(content), "token: ${WEBEX_TOKEN}\n    room_id: ${WEBEX_ROOM_ID}\n") {
		t.Errorf("webex config:\n%s", content)
	}

	if r := run(t, "skype\n", "init", "--output", "other.yaml"); r.code != exitConfig {
		t.Errorf("bad platform: %+v", r)
	}
}

func TestValidate(t *testing.T) {
	isolate(t)

	if r := run(t, "", "validate"); r.code != exitConfig || !strings.Contains(r.stderr, "no config file found") {
		t.Errorf("no config: %+v", r)
	}

	writeFile(t, "bad.yaml", "targets:\n  - name: x\n    type: skype\n    webhook: ${H}\n")
	if r := run(t, "", "validate", "bad.yaml"); r.code != exitConfig || !strings.Contains(r.stderr, `type "skype"`) {
		t.Errorf("bad type: %+v", r)
	}

	writeFile(t, "nomatch.yaml", "selection:\n  categories: [nope]\ntargets:\n  - name: x\n    type: webhook\n    url: http://x\n")
	if r := run(t, "", "validate", "nomatch.yaml"); r.code != exitConfig || !strings.Contains(r.stderr, "no questions match") {
		t.Errorf("no match: %+v", r)
	}

	writeFile(t, "badtemplate.yaml", "message:\n  template: '{{ .Nope'\ntargets:\n  - name: x\n    type: webhook\n    url: http://x\n")
	if r := run(t, "", "validate", "badtemplate.yaml"); r.code != exitConfig || !strings.Contains(r.stderr, "message.template") {
		t.Errorf("bad template: %+v", r)
	}

	if r := run(t, "", "validate", "a.yaml", "b.yaml"); r.code != exitConfig {
		t.Errorf("two paths: %+v", r)
	}
	if r := run(t, "", "validate", "missing.yaml"); r.code != exitConfig {
		t.Errorf("missing file: %+v", r)
	}
}

const teamPack = `category: team
name: Our Team
prefix: team
questions:
  - id: team-0001
    text: "What is the best team lunch spot?"
    type: open
    tags: [anything-goes]
    sfw: safe
    locale: en
    added: 2026-10-14
  - id: team-0002
    text: "Standup or async updates?"
    type: this-or-that
    options: [Standup, Async]
    sfw: safe
    locale: en
    added: 2026-10-14
`

func TestValidatePack(t *testing.T) {
	isolate(t)

	writeFile(t, "team.yaml", teamPack)
	if r := run(t, "", "validate", "team.yaml"); r.code != exitOK || !strings.Contains(r.stdout, "has 2 questions") {
		t.Errorf("good pack: %+v", r)
	}

	writeFile(t, "team.yaml", strings.Replace(teamPack, "type: open", "type: quiz", 1))
	if r := run(t, "", "validate", "team.yaml"); r.code != exitConfig || !strings.Contains(r.stderr, `type "quiz"`) {
		t.Errorf("bad pack: %+v", r)
	}
}

// postConfig writes a config with one slack target pointing at url.
func postConfig(t *testing.T, mode, extra string) {
	t.Helper()
	writeFile(t, "cuppa.yaml", `selection:
  mode: `+mode+`
  categories: [team]
data:
  extra_questions: [team.yaml]
history:
  path: h.json
targets:
  - name: one
    type: slack
    webhook: ${HOOK_ONE}
`+extra)
	writeFile(t, "team.yaml", teamPack)
}

func TestPost(t *testing.T) {
	isolate(t)
	chat := newChat(t, http.StatusOK)
	t.Setenv("HOOK_ONE", chat.url)
	postConfig(t, "no-repeat", "")

	r := run(t, "", "post")
	if r.code != exitOK {
		t.Fatalf("post: %+v", r)
	}
	bodies := chat.received()
	if len(bodies) != 1 || !strings.Contains(bodies[0], `"text":"☕ *Today's watercooler:*`) {
		t.Errorf("chat got %v", bodies)
	}

	// no-repeat: second post must pick the other question
	run(t, "", "post")
	bodies = chat.received()
	if len(bodies) != 2 || bodies[0] == bodies[1] {
		t.Errorf("repeat within a cycle: %v", bodies)
	}

	// third post: every question used, so a new cycle starts
	run(t, "", "post")
	h := history.NewFile("h.json")
	used, _ := h.Used()
	cycles, _ := h.Cycles()
	if len(used) != 1 || cycles != 1 {
		t.Errorf("after a full cycle: used %d, cycles %d", len(used), cycles)
	}
}

func TestPostDryRunChangesNothing(t *testing.T) {
	isolate(t)
	chat := newChat(t, http.StatusOK)
	t.Setenv("HOOK_ONE", chat.url)
	postConfig(t, "no-repeat", "")

	r := run(t, "", "post", "--dry-run")
	if r.code != exitOK || !strings.Contains(r.stdout, "would post to one:") {
		t.Fatalf("dry run: %+v", r)
	}
	if len(chat.received()) != 0 {
		t.Error("dry run sent a message")
	}
	if _, err := os.Stat("h.json"); err == nil {
		t.Error("dry run wrote the history")
	}
}

func TestPostFailures(t *testing.T) {
	isolate(t)
	good := newChat(t, http.StatusOK)
	bad := newChat(t, http.StatusNotFound)
	t.Setenv("HOOK_ONE", good.url)
	t.Setenv("HOOK_TWO", bad.url)
	postConfig(t, "no-repeat", `  - name: two
    type: gchat
    webhook: ${HOOK_TWO}
`)

	// one of two targets fails: exit 3, but the good one still got it
	// and the question counts as used
	r := run(t, "", "post")
	if r.code != exitPostFailed || !strings.Contains(r.stderr, "posted to 1 of 2 targets") {
		t.Errorf("partial failure: %+v", r)
	}
	if used, _ := history.NewFile("h.json").Used(); len(used) != 1 {
		t.Errorf("history after partial failure: %v", used)
	}

	// only the failing target: nothing saved
	if err := history.NewFile("h.json").Reset(); err != nil {
		t.Fatal(err)
	}
	if r := run(t, "", "post", "--target", "two"); r.code != exitPostFailed {
		t.Errorf("all failed: %+v", r)
	}
	if used, _ := history.NewFile("h.json").Used(); len(used) != 0 {
		t.Errorf("history after total failure: %v", used)
	}

	if r := run(t, "", "post", "--target", "three"); r.code != exitConfig || !strings.Contains(r.stderr, `no target named "three"`) {
		t.Errorf("unknown target: %+v", r)
	}
}

// Each chat app gets bold text in its own style: *x* for Slack,
// **x** for markdown apps like Discord, Mattermost, Teams and Webex.
func TestPostEveryTargetType(t *testing.T) {
	isolate(t)
	chat := newChat(t, http.StatusOK)
	t.Setenv("HOOK_ONE", chat.url)
	postConfig(t, "daily", `  - name: discord
    type: discord
    webhook: ${HOOK_ONE}
  - name: mattermost
    type: mattermost
    webhook: ${HOOK_ONE}
  - name: teams
    type: teams
    webhook: ${HOOK_ONE}
`)

	r := run(t, "", "post")
	if r.code != exitOK {
		t.Fatalf("post: %+v", r)
	}
	bodies := chat.received()
	if len(bodies) != 4 {
		t.Fatalf("chat got %d bodies: %v", len(bodies), bodies)
	}
	if !strings.Contains(bodies[0], `"text":"☕ *Today's watercooler:*`) {
		t.Errorf("slack: %s", bodies[0])
	}
	for _, body := range bodies[1:] {
		if !strings.Contains(body, `☕ **Today's watercooler:**`) {
			t.Errorf("markdown target: %s", body)
		}
	}

	// webex posts to its own API, so only check that it builds
	writeFile(t, "webex.yaml", "targets:\n  - name: w\n    type: webex\n    token: ${HOOK_ONE}\n    room_id: room-1\n")
	if r := run(t, "", "validate", "webex.yaml"); r.code != exitOK {
		t.Errorf("webex: %+v", r)
	}
}

func TestPostConfigErrors(t *testing.T) {
	isolate(t)
	if r := run(t, "", "post"); r.code != exitConfig || !strings.Contains(r.stderr, "no config file found") {
		t.Errorf("no config: %+v", r)
	}

	tests := map[string]string{
		"no targets":     "version: 1\n",
		"no name":        "targets:\n  - type: slack\n    webhook: http://x\n",
		"duplicate name": "targets:\n  - name: a\n    type: slack\n    webhook: http://x\n  - name: a\n    type: slack\n    webhook: http://x\n",
		"missing env":    "targets:\n  - name: a\n    type: slack\n    webhook: ${UNSET_HOOK}\n",
		"empty url":      "targets:\n  - name: a\n    type: webhook\n",
		"webex no room":  "targets:\n  - name: a\n    type: webex\n    token: ${UNSET_TOKEN}\n",
		"bad body":       "targets:\n  - name: a\n    type: webhook\n    url: http://x\n    body: '{{ .Nope'\n",
		"bad poll":       "message:\n  poll_template: '{{ .Nope'\ntargets:\n  - name: a\n    type: webhook\n    url: http://x\n",
		"broken pack":    "data:\n  extra_questions: [missing.yaml]\ntargets:\n  - name: a\n    type: webhook\n    url: http://x\n",
	}
	for name, content := range tests {
		t.Run(name, func(t *testing.T) {
			writeFile(t, "cuppa.yaml", content)
			if r := run(t, "", "post"); r.code != exitConfig {
				t.Errorf("got %+v", r)
			}
		})
	}
}

func TestPackOverridesBuiltInQuestion(t *testing.T) {
	isolate(t)
	writeFile(t, "cuppa.yaml", "data:\n  extra_questions: [mine.yaml]\n")
	writeFile(t, "mine.yaml", `category: icebreakers
name: Icebreakers
prefix: ice
questions:
  - id: ice-0001
    text: "What is our own version of question one?"
    type: open
    sfw: safe
    locale: en
    added: 2026-10-14
`)
	r := run(t, "", "list", "--category", "icebreakers")
	if r.code != exitOK || !strings.Contains(r.stdout, "our own version") || !strings.Contains(r.stderr, "replaces a built-in question") {
		t.Errorf("got %+v", r)
	}
}

func TestRenderErrors(t *testing.T) {
	r, err := newRenderer(configMessage("{{ .Text }}", "{{ index .Emojis 9 }}"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.render(cuppa.Question{Text: "Coffee or tea?", Options: []string{"a", "b"}}, "*"); err == nil {
		t.Error("want error from a template that fails to run")
	}
}

func TestSplitList(t *testing.T) {
	if got := splitList(" a, b,,c "); strings.Join(got, "|") != "a|b|c" {
		t.Errorf("got %v", got)
	}
	if got := splitList(""); got != nil {
		t.Errorf("got %v", got)
	}
}

func configMessage(template, poll string) config.Message {
	return config.Message{Template: template, PollTemplate: poll}
}

func TestPostJSONAndSelectionFlags(t *testing.T) {
	isolate(t)
	chat := newChat(t, http.StatusOK)
	t.Setenv("HOOK_ONE", chat.url)
	postConfig(t, "no-repeat", "")

	// flags override the config: random mode, only this-or-that questions
	r := run(t, "", "post", "--json", "--mode", "random", "--category", "team", "--tag", "nope-no-such-tag")
	if r.code != exitNoMatch {
		t.Fatalf("tag filter should leave nothing: %+v", r)
	}

	r = run(t, "", "post", "--json", "--mode", "random")
	var q cuppa.Question
	if err := json.Unmarshal([]byte(r.stdout), &q); err != nil || q.Category != "team" {
		t.Fatalf("post --json: %+v %v", r, err)
	}
	// random mode never touches the history
	if _, err := os.Stat("h.json"); err == nil {
		t.Error("random mode wrote the history")
	}

	r = run(t, "", "post", "--json", "--dry-run")
	if err := json.Unmarshal([]byte(r.stdout), &q); err != nil || q.ID == "" {
		t.Fatalf("dry run --json: %+v %v", r, err)
	}
	if len(chat.received()) != 1 {
		t.Errorf("chat got %d messages, want 1", len(chat.received()))
	}

	if r := run(t, "", "post", "--mode", "weekly"); r.code != exitConfig {
		t.Errorf("bad mode: %+v", r)
	}
}

// fakeClock lets daemon tests run for days in an instant. Each sleep
// moves the clock forward; the first one can jump further, like a
// laptop waking up. Sleeping past end stops the daemon.
func fakeClock(t *testing.T, end time.Time, firstSleep time.Duration) {
	t.Helper()
	now := fixedNow
	oldNow, oldSleep := timeNow, sleep
	timeNow = func() time.Time { return now }
	sleep = func(_ context.Context, d time.Duration) error {
		if firstSleep > 0 {
			d, firstSleep = firstSleep, 0
		}
		now = now.Add(d)
		if now.After(end) {
			return context.Canceled
		}
		return nil
	}
	t.Cleanup(func() { timeNow, sleep = oldNow, oldSleep })
}

// daemonConfig is postConfig with a daily 10:00 UTC schedule that
// skips 2026-10-15. fixedNow is Wednesday 2026-10-14 09:00 UTC.
func daemonConfig(t *testing.T, mode string) {
	t.Helper()
	postConfig(t, mode, "schedule:\n  cron: \"0 10 * * *\"\n  timezone: UTC\n  skip_dates: [\"2026-10-15\"]\n")
}

func TestDaemon(t *testing.T) {
	isolate(t)
	chat := newChat(t, http.StatusOK)
	t.Setenv("HOOK_ONE", chat.url)
	daemonConfig(t, "no-repeat")
	fakeClock(t, time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC), 0)

	// posts on the 14th and the 16th; the 15th is a skip date
	r := run(t, "", "daemon")
	if r.code != exitOK || !strings.Contains(r.stderr, "at=2026-10-16T10:00:00Z") || !strings.Contains(r.stderr, "stopped") {
		t.Errorf("daemon: %+v", r)
	}
	bodies := chat.received()
	if len(bodies) != 2 || bodies[0] == bodies[1] {
		t.Errorf("chat got %v", bodies)
	}
	if used, _ := history.NewFile("h.json").Used(); len(used) != 2 {
		t.Errorf("history: %v", used)
	}
}

func TestDaemonSkipsMissedPost(t *testing.T) {
	isolate(t)
	chat := newChat(t, http.StatusOK)
	t.Setenv("HOOK_ONE", chat.url)
	daemonConfig(t, "daily")
	// the computer sleeps from 09:00 to 12:00, past the 10:00 post
	fakeClock(t, time.Date(2026, 10, 16, 9, 0, 0, 0, time.UTC), 3*time.Hour)

	r := run(t, "", "daemon")
	if r.code != exitOK || !strings.Contains(r.stderr, "missed a post time") {
		t.Errorf("daemon: %+v", r)
	}
	if bodies := chat.received(); len(bodies) != 0 {
		t.Errorf("a missed post went out: %v", bodies)
	}
}

func TestDaemonKeepsGoingAfterFailures(t *testing.T) {
	isolate(t)
	chat := newChat(t, http.StatusNotFound)
	t.Setenv("HOOK_ONE", chat.url)
	daemonConfig(t, "daily")
	fakeClock(t, time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC), 0)

	r := run(t, "", "daemon")
	if r.code != exitOK || strings.Count(r.stderr, "post did not fully work") != 2 {
		t.Errorf("daemon: %+v", r)
	}
}

func TestDaemonDryRun(t *testing.T) {
	isolate(t)
	chat := newChat(t, http.StatusOK)
	t.Setenv("HOOK_ONE", chat.url)
	daemonConfig(t, "no-repeat")
	fakeClock(t, time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC), 0)

	r := run(t, "", "daemon", "--dry-run")
	if r.code != exitOK || !strings.Contains(r.stderr, "would post") {
		t.Errorf("daemon: %+v", r)
	}
	if len(chat.received()) != 0 {
		t.Error("dry run sent something")
	}
	if used, _ := history.NewFile("h.json").Used(); len(used) != 0 {
		t.Errorf("dry run saved history: %v", used)
	}
}

func TestDaemonConfigErrors(t *testing.T) {
	isolate(t)
	if r := run(t, "", "daemon"); r.code != exitConfig || !strings.Contains(r.stderr, "no config file found") {
		t.Errorf("no config: %+v", r)
	}

	t.Setenv("HOOK_ONE", "http://x")
	postConfig(t, "daily", "")
	if r := run(t, "", "daemon"); r.code != exitConfig || !strings.Contains(r.stderr, "schedule.cron is empty") {
		t.Errorf("no cron: %+v", r)
	}

	postConfig(t, "daily", "schedule:\n  cron: \"0 10 * * MON\"\n")
	if r := run(t, "", "daemon"); r.code != exitConfig || !strings.Contains(r.stderr, "day of week") {
		t.Errorf("bad cron: %+v", r)
	}
	if r := run(t, "", "validate"); r.code != exitConfig {
		t.Errorf("validate bad cron: %+v", r)
	}

	// Feb 30 never comes
	postConfig(t, "daily", "schedule:\n  cron: \"0 10 30 2 *\"\n")
	if r := run(t, "", "daemon"); r.code != exitConfig || !strings.Contains(r.stderr, "no post time") {
		t.Errorf("never fires: %+v", r)
	}
	if r := run(t, "", "validate"); r.code != exitConfig {
		t.Errorf("validate never fires: %+v", r)
	}

	// fixedNow is 14:00 in Karachi, so the next weekday 10:00 is tomorrow
	postConfig(t, "daily", "schedule:\n  cron: \"0 10 * * 1-5\"\n  timezone: Asia/Karachi\n")
	if r := run(t, "", "validate"); r.code != exitOK || !strings.Contains(r.stdout, "next daemon post: Thu 2026-10-15 10:00 PKT") {
		t.Errorf("validate: %+v", r)
	}
}

func TestRealSleepStopsOnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := sleep(ctx, time.Hour); err == nil {
		t.Error("want an error from a cancelled sleep")
	}
	if err := sleep(context.Background(), time.Millisecond); err != nil {
		t.Error(err)
	}
}
