package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"slices"
	"strings"

	"github.com/kashifkhan0771/cuppa/internal/config"
)

// runInit asks a few questions and writes a starter cuppa.yaml.
// Pressing enter takes the default shown in [brackets].
func runInit(a *app, args []string) error {
	flags := a.newFlagSet("init", "init [--output cuppa.yaml] [--force]")
	output := flags.String("output", "cuppa.yaml", "file to write")
	force := flags.Bool("force", false, "overwrite the file if it exists")
	if err := a.parse(flags, args); err != nil {
		return err
	}

	if _, err := os.Stat(*output); err == nil && !*force {
		return configError(fmt.Errorf("%s already exists; use --force to overwrite it", *output))
	} else if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}

	in := bufio.NewScanner(a.stdin)
	platform, err := a.ask(in, "Chat platform ("+strings.Join(targetTypes, ", ")+")", "slack", targetTypes)
	if err != nil {
		return err
	}
	// ask for the env vars that will hold the secrets; webex needs
	// a bot token and a room ID, the others one webhook URL
	secrets := []secret{{key: "webhook", hint: "webhook URL", env: strings.ToUpper(platform) + "_WEBHOOK"}}
	switch platform {
	case "webhook":
		secrets[0].key = "url"
	case "webex":
		secrets = []secret{{key: "token", hint: "bot token", env: "WEBEX_TOKEN"}, {key: "room_id", hint: "room ID", env: "WEBEX_ROOM_ID"}}
	}
	for i, s := range secrets {
		if secrets[i].env, err = a.ask(in, "Environment variable that will hold the "+s.hint, s.env, nil); err != nil {
			return err
		}
	}
	mode, err := a.ask(in, "How to pick questions ("+strings.Join(config.Modes, ", ")+")", config.ModeDaily, config.Modes)
	if err != nil {
		return err
	}

	content := starterConfig(platform, mode, secrets)
	if err := os.WriteFile(*output, []byte(content), 0o600); err != nil {
		return err
	}
	fmt.Fprintf(a.stdout, "\nwrote %s\n\nnext steps:\n", *output)
	for _, s := range secrets {
		fmt.Fprintf(a.stdout, "  export %s='<your %s>'\n", s.env, s.hint)
	}
	fmt.Fprintf(a.stdout, "  cuppa post --dry-run --config %s\n", *output)
	return nil
}

// secret is one config key that init fills from an env var.
type secret struct {
	key  string // config key, like webhook
	hint string // what it holds, for the questions
	env  string // env var that holds it
}

// ask prints a question and reads one answer line. An empty answer
// (or no more input) takes the default. If allowed is set, the answer
// must be one of those values.
func (a *app) ask(in *bufio.Scanner, question, fallback string, allowed []string) (string, error) {
	fmt.Fprintf(a.stdout, "%s [%s]: ", question, fallback)
	answer := fallback
	if in.Scan() {
		if text := strings.TrimSpace(in.Text()); text != "" {
			answer = text
		}
	}
	if allowed != nil && !slices.Contains(allowed, answer) {
		return "", configError(fmt.Errorf("%q must be one of %s", answer, strings.Join(allowed, ", ")))
	}
	return answer, nil
}

// starterConfig is the text of a new cuppa.yaml, with comments
// so people can see what else they can change.
func starterConfig(platform, mode string, secrets []secret) string {
	var keys strings.Builder
	for _, s := range secrets {
		fmt.Fprintf(&keys, "    %s: ${%s}\n", s.key, s.env)
	}
	return fmt.Sprintf(`version: 1

selection:
  # daily, random or no-repeat
  mode: %s
  # daily mode: change the seed to get a different question order
  seed: my-team
  # categories: [icebreakers, this-or-that]
  # safe, or casual for relaxed teams
  sfw: safe

# whose midnight starts a new day (daily mode)
# schedule:
#   timezone: Asia/Karachi

# your own private questions
# data:
#   extra_questions: [./our-questions.yaml]

targets:
  - name: team-%s
    type: %s
    # keep secrets in environment variables, not in this file
%s`, mode, platform, platform, keys.String())
}
