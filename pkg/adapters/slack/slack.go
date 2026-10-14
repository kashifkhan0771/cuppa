// Package slack posts to a Slack incoming webhook.
// Slack reads the text as mrkdwn, so *bold* and _italic_ work.
package slack

import (
	"context"

	"github.com/kashifkhan0771/cuppa/pkg/adapters"
)

// Adapter posts to one Slack incoming webhook.
type Adapter struct {
	webhookURL string
}

// New returns an Adapter for a webhook URL like
// https://hooks.slack.com/services/T000/B000/XXXX.
func New(webhookURL string) *Adapter {
	return &Adapter{webhookURL: webhookURL}
}

type payload struct {
	Text string `json:"text"`
}

func (a *Adapter) Post(ctx context.Context, m adapters.Message) error {
	return adapters.PostJSON(ctx, a.webhookURL, payload{Text: m.Rendered})
}
