// Package gchat posts to a Google Chat space incoming webhook.
// Google Chat reads simple formatting like *bold* and _italic_.
package gchat

import (
	"context"

	"github.com/kashifkhan0771/cuppa/pkg/adapters"
)

// Adapter posts to one Google Chat space webhook.
type Adapter struct {
	webhookURL string
}

// New returns an Adapter for a webhook URL like
// https://chat.googleapis.com/v1/spaces/AAAA/messages?key=...&token=....
func New(webhookURL string) *Adapter {
	return &Adapter{webhookURL: webhookURL}
}

type payload struct {
	Text string `json:"text"`
}

func (a *Adapter) Post(ctx context.Context, m adapters.Message) error {
	return adapters.PostJSON(ctx, a.webhookURL, payload{Text: m.Rendered})
}
