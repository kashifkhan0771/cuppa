// Package webhook posts to any HTTP endpoint, with a request body
// built from a Go template. Use it for chat services that have no
// adapter of their own.
package webhook

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"text/template"

	"github.com/kashifkhan0771/cuppa/pkg/adapters"
)

// DefaultBody sends the message as {"text": "..."}, which many
// chat webhooks accept as is.
const DefaultBody = `{"text": {{ json .Rendered }}}`

// Options configure the webhook.
//
// Body is a Go template. It sees .Rendered (the message) and .Question
// (all question fields), plus a json function that turns any value
// into a safely quoted JSON value.
type Options struct {
	URL     string
	Method  string            // default POST
	Headers map[string]string // extra request headers
	Body    string            // default DefaultBody
}

// Adapter posts to one HTTP endpoint.
type Adapter struct {
	options Options
	body    *template.Template
}

// New checks the options and compiles the body template.
func New(options Options) (*Adapter, error) {
	if options.Method == "" {
		options.Method = http.MethodPost
	}
	if options.Body == "" {
		options.Body = DefaultBody
	}

	body, err := template.New("body").Funcs(template.FuncMap{"json": toJSON}).Parse(options.Body)
	if err != nil {
		return nil, fmt.Errorf("webhook body template: %w", err)
	}
	return &Adapter{options: options, body: body}, nil
}

func (a *Adapter) Post(ctx context.Context, m adapters.Message) error {
	var body bytes.Buffer
	if err := a.body.Execute(&body, m); err != nil {
		return fmt.Errorf("webhook body template: %w", err)
	}
	return adapters.Send(ctx, a.options.Method, a.options.URL, a.options.Headers, body.Bytes())
}

// toJSON is the template's json function.
func toJSON(v any) (string, error) {
	raw, err := adapters.MarshalJSON(v)
	return string(raw), err
}
