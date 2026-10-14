// Package teams posts to a Microsoft Teams channel through a
// Workflows webhook (the "Send webhook alerts to a channel" template).
// The message goes out as an Adaptive Card, which reads **bold**.
package teams

import (
	"context"
	"strings"

	"github.com/kashifkhan0771/cuppa/pkg/adapters"
)

// Adapter posts to one Teams Workflows webhook.
type Adapter struct {
	webhookURL string
}

// New returns an Adapter for the URL Teams shows when you create
// the workflow.
func New(webhookURL string) *Adapter {
	return &Adapter{webhookURL: webhookURL}
}

type payload struct {
	Type        string       `json:"type"`
	Attachments []attachment `json:"attachments"`
}

type attachment struct {
	ContentType string `json:"contentType"`
	Content     card   `json:"content"`
}

type card struct {
	Schema  string      `json:"$schema"`
	Type    string      `json:"type"`
	Version string      `json:"version"`
	Body    []textBlock `json:"body"`
}

type textBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
	Wrap bool   `json:"wrap"`
}

func (a *Adapter) Post(ctx context.Context, m adapters.Message) error {
	// each line gets its own text block, so poll options always show
	// on their own lines however the card treats line breaks
	var body []textBlock
	for line := range strings.SplitSeq(m.Rendered, "\n") {
		if strings.TrimSpace(line) != "" {
			body = append(body, textBlock{Type: "TextBlock", Text: line, Wrap: true})
		}
	}

	return adapters.PostJSON(ctx, a.webhookURL, payload{
		Type: "message",
		Attachments: []attachment{{
			ContentType: "application/vnd.microsoft.card.adaptive",
			Content: card{
				Schema:  "http://adaptivecards.io/schemas/adaptive-card.json",
				Type:    "AdaptiveCard",
				Version: "1.2",
				Body:    body,
			},
		}},
	})
}
