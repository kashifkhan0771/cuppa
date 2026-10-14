package cli

import (
	"encoding/json"
	"fmt"

	"github.com/kashifkhan0771/cuppa/internal/history"
)

// runNext prints a question. In no-repeat mode it marks the question
// as used, so scripts can do their own posting.
func runNext(a *app, args []string) error {
	fs := a.newFlagSet("next", "next [--category a,b] [--tag x,y] [--mode daily|random|no-repeat] [--json]")
	categories := fs.String("category", "", "only these categories, comma separated")
	tags := fs.String("tag", "", "only questions with one of these tags, comma separated")
	mode := fs.String("mode", "", "daily, random or no-repeat (default from config)")
	asJSON := fs.Bool("json", false, "print the question as JSON instead of the message")
	if err := a.parse(fs, args); err != nil {
		return err
	}
	if err := a.loadConfig(); err != nil {
		return err
	}

	selection, err := a.selection(*categories, *tags, *mode)
	if err != nil {
		return err
	}
	dataset, err := a.questionsFor(selection)
	if err != nil {
		return err
	}
	q, err := a.pick(dataset, selection, history.NewFile(a.cfg.History.Path))
	if err != nil {
		return err
	}

	if *asJSON {
		enc := json.NewEncoder(a.stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(q)
	}
	r, err := newRenderer(a.cfg.Message)
	if err != nil {
		return err
	}
	message, err := r.render(q)
	if err != nil {
		return err
	}
	fmt.Fprintln(a.stdout, message)
	return nil
}
