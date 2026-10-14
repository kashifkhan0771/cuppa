// Package webex posts to a Webex space as a bot. Webex has no
// incoming webhooks, so it needs a bot token and the space's room ID.
// Webex reads markdown, so bold is **bold**.
package webex

import (
	"context"
	"net/http"

	"github.com/kashifkhan0771/cuppa/pkg/adapters"
)

// messagesURL is the Webex API for sending a message.
// Tests point it at a fake server.
var messagesURL = "https://webexapis.com/v1/messages"

// Options configure the Webex adapter.
type Options struct {
	Token  string // bot access token
	RoomID string // the space to post in; the bot must be a member
}

// Adapter posts to one Webex space.
type Adapter struct {
	options Options
}

// New returns an Adapter for one bot and space.
func New(options Options) *Adapter {
	return &Adapter{options: options}
}

type payload struct {
	RoomID   string `json:"roomId"`
	Markdown string `json:"markdown"`
}

func (a *Adapter) Post(ctx context.Context, m adapters.Message) error {
	body, err := adapters.MarshalJSON(payload{RoomID: a.options.RoomID, Markdown: m.Rendered})
	if err != nil {
		return err
	}
	headers := map[string]string{"Authorization": "Bearer " + a.options.Token}
	return adapters.Send(ctx, http.MethodPost, messagesURL, headers, body)
}
