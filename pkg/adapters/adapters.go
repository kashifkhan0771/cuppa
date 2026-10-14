// Package adapters posts a question to team chat. Each chat service has
// its own sub-package (slack, gchat, webhook); they share the sending,
// retry and secret-hiding code in this package.
package adapters

import (
	"context"

	"github.com/kashifkhan0771/cuppa/pkg/cuppa"
)

// Message is what gets posted.
type Message struct {
	Question cuppa.Question
	Rendered string // the question after the message template was applied
}

// Adapter posts a message to one chat service.
type Adapter interface {
	Post(ctx context.Context, m Message) error
}
