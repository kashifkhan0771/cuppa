// Package mattermost posts to a Mattermost incoming webhook.
// Mattermost reads markdown, so bold is **bold**.
package mattermost

import (
	"context"

	"github.com/kashifkhan0771/cuppa/pkg/adapters"
)

// Adapter posts to one Mattermost incoming webhook.
type Adapter struct {
	webhookURL string
}

// New returns an Adapter for a webhook URL like
// https://chat.example.com/hooks/xxxx.
func New(webhookURL string) *Adapter {
	return &Adapter{webhookURL: webhookURL}
}

type payload struct {
	Text string `json:"text"`
}

func (a *Adapter) Post(ctx context.Context, m adapters.Message) error {
	return adapters.PostJSON(ctx, a.webhookURL, payload{Text: m.Rendered})
}
