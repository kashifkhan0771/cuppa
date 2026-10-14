package cli

import (
	"fmt"
	"slices"
	"strings"

	"github.com/kashifkhan0771/cuppa/internal/config"
	"github.com/kashifkhan0771/cuppa/pkg/adapters"
	"github.com/kashifkhan0771/cuppa/pkg/adapters/discord"
	"github.com/kashifkhan0771/cuppa/pkg/adapters/gchat"
	"github.com/kashifkhan0771/cuppa/pkg/adapters/mattermost"
	"github.com/kashifkhan0771/cuppa/pkg/adapters/slack"
	"github.com/kashifkhan0771/cuppa/pkg/adapters/teams"
	"github.com/kashifkhan0771/cuppa/pkg/adapters/webex"
	"github.com/kashifkhan0771/cuppa/pkg/adapters/webhook"
)

// targetTypes lists the supported target types.
// To add an adapter: add a package under pkg/adapters and a case below.
var targetTypes = []string{"slack", "gchat", "teams", "discord", "mattermost", "webex", "webhook"}

// markdownTypes write bold as **text**. The others (Slack, Google Chat,
// and plain webhooks) use *text*, which markdown apps show as italic.
var markdownTypes = []string{"teams", "discord", "mattermost", "webex"}

// boldMark is what goes around bold text for a target type.
func boldMark(targetType string) string {
	if slices.Contains(markdownTypes, targetType) {
		return "**"
	}
	return "*"
}

type target struct {
	name    string
	bold    string // see boldMark
	adapter adapters.Adapter
}

// buildTargets makes an adapter for each target in the config.
// With only set, it builds just that one target.
func (a *app) buildTargets(only string) ([]target, error) {
	var targets []target
	seen := map[string]bool{}
	for i, t := range a.cfg.Targets {
		if t.Name == "" {
			return nil, configError(fmt.Errorf("targets[%d] needs a name", i))
		}
		if seen[t.Name] {
			return nil, configError(fmt.Errorf("target name %q is used twice", t.Name))
		}
		seen[t.Name] = true
		if only != "" && t.Name != only {
			continue
		}

		adapter, err := a.newAdapter(t)
		if err != nil {
			return nil, configError(fmt.Errorf("target %q: %w", t.Name, err))
		}
		targets = append(targets, target{name: t.Name, bold: boldMark(t.Type), adapter: adapter})
	}

	if only != "" && len(targets) == 0 {
		return nil, configError(fmt.Errorf("no target named %q in the config", only))
	}
	if len(targets) == 0 {
		return nil, configError(fmt.Errorf("the config has no targets to post to"))
	}
	return targets, nil
}

func (a *app) newAdapter(t config.Target) (adapters.Adapter, error) {
	switch t.Type {
	case "slack", "gchat", "teams", "discord", "mattermost":
		if t.Webhook == "" {
			return nil, fmt.Errorf("webhook is empty%s", a.missingEnvHint())
		}
		switch t.Type {
		case "slack":
			return slack.New(t.Webhook), nil
		case "gchat":
			return gchat.New(t.Webhook), nil
		case "teams":
			return teams.New(t.Webhook), nil
		case "discord":
			return discord.New(t.Webhook), nil
		default:
			return mattermost.New(t.Webhook), nil
		}
	case "webex":
		if t.Token == "" || t.RoomID == "" {
			return nil, fmt.Errorf("webex needs both token and room_id%s", a.missingEnvHint())
		}
		return webex.New(webex.Options{Token: t.Token, RoomID: t.RoomID}), nil
	case "webhook":
		if t.URL == "" {
			return nil, fmt.Errorf("url is empty%s", a.missingEnvHint())
		}
		return webhook.New(webhook.Options{URL: t.URL, Method: t.Method, Headers: t.Headers, Body: t.Body})
	default:
		return nil, fmt.Errorf("type %q must be one of %s", t.Type, strings.Join(targetTypes, ", "))
	}
}
