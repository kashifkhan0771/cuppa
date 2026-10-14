package cli

import (
	"fmt"
	"strings"

	"github.com/kashifkhan0771/cuppa/internal/config"
	"github.com/kashifkhan0771/cuppa/pkg/adapters"
	"github.com/kashifkhan0771/cuppa/pkg/adapters/gchat"
	"github.com/kashifkhan0771/cuppa/pkg/adapters/slack"
	"github.com/kashifkhan0771/cuppa/pkg/adapters/webhook"
)

// targetTypes lists the supported target types.
// To add an adapter: add a package under pkg/adapters and a case below.
var targetTypes = []string{"slack", "gchat", "webhook"}

type target struct {
	name    string
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
		targets = append(targets, target{name: t.Name, adapter: adapter})
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
	case "slack", "gchat":
		if t.Webhook == "" {
			return nil, fmt.Errorf("webhook is empty%s", a.missingEnvHint())
		}
		if t.Type == "slack" {
			return slack.New(t.Webhook), nil
		}
		return gchat.New(t.Webhook), nil
	case "webhook":
		if t.URL == "" {
			return nil, fmt.Errorf("url is empty%s", a.missingEnvHint())
		}
		return webhook.New(webhook.Options{URL: t.URL, Method: t.Method, Headers: t.Headers, Body: t.Body})
	default:
		return nil, fmt.Errorf("type %q must be one of %s", t.Type, strings.Join(targetTypes, ", "))
	}
}
