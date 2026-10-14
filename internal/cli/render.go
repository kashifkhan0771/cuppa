package cli

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"

	"github.com/kashifkhan0771/cuppa/internal/config"
	"github.com/kashifkhan0771/cuppa/pkg/cuppa"
)

// optionEmojis number poll options in the message. Questions have
// at most 6 options, so 6 are enough.
var optionEmojis = []string{"1️⃣", "2️⃣", "3️⃣", "4️⃣", "5️⃣", "6️⃣"}

// renderer turns a question into the message text.
type renderer struct {
	plain *template.Template // open questions
	poll  *template.Template // questions with options
}

func newRenderer(m config.Message) (*renderer, error) {
	plain, err := template.New("template").Parse(m.Template)
	if err != nil {
		return nil, configError(fmt.Errorf("message.template: %w", err))
	}
	poll, err := template.New("poll_template").Parse(m.PollTemplate)
	if err != nil {
		return nil, configError(fmt.Errorf("message.poll_template: %w", err))
	}
	return &renderer{plain: plain, poll: poll}, nil
}

// templateData is what templates see: every question field
// (.Text, .Options, .Category, ...) plus .Emojis.
type templateData struct {
	cuppa.Question
	Emojis []string
}

func (r *renderer) render(q cuppa.Question) (string, error) {
	t := r.plain
	if len(q.Options) > 0 {
		t = r.poll
	}

	var out bytes.Buffer
	if err := t.Execute(&out, templateData{Question: q, Emojis: optionEmojis}); err != nil {
		return "", configError(fmt.Errorf("message template: %w", err))
	}
	return strings.TrimSpace(out.String()), nil
}
