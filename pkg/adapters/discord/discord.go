// Package discord posts to a Discord channel webhook.
// Discord reads markdown, so bold is **bold**.
package discord

import (
	"context"

	"github.com/kashifkhan0771/cuppa/pkg/adapters"
)

// Adapter posts to one Discord webhook.
type Adapter struct {
	webhookURL string
}

// New returns an Adapter for a webhook URL like
// https://discord.com/api/webhooks/123/XXXX.
func New(webhookURL string) *Adapter {
	return &Adapter{webhookURL: webhookURL}
}

type payload struct {
	Content         string          `json:"content"`
	AllowedMentions allowedMentions `json:"allowed_mentions"`
}

// allowedMentions with an empty Parse list turns off every ping,
// so text like @everyone in a question can't notify the whole server.
type allowedMentions struct {
	Parse []string `json:"parse"`
}

func (a *Adapter) Post(ctx context.Context, m adapters.Message) error {
	body := payload{Content: m.Rendered, AllowedMentions: allowedMentions{Parse: []string{}}}
	return adapters.PostJSON(ctx, a.webhookURL, body)
}
