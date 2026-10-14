package cli

import (
	"fmt"

	"github.com/kashifkhan0771/cuppa/internal/history"
)

// runNext prints a question. In no-repeat mode it marks the question
// as used, so scripts can do their own posting.
func runNext(a *app, args []string) error {
	fs := a.newFlagSet("next", "next [--category a,b] [--tag x,y] [--mode daily|random|no-repeat] [--json]")
	selectionFlags := addSelectionFlags(fs)
	asJSON := fs.Bool("json", false, "print the question as JSON instead of the message")
	if err := a.parse(fs, args); err != nil {
		return err
	}
	if err := a.loadConfig(); err != nil {
		return err
	}

	selection, err := a.selectionFrom(selectionFlags)
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
		return printJSON(a, q)
	}
	r, err := newRenderer(a.cfg.Message)
	if err != nil {
		return err
	}
	// the terminal shows *bold*, like Slack
	message, err := r.render(q, boldMark("slack"))
	if err != nil {
		return err
	}
	fmt.Fprintln(a.stdout, message)
	return nil
}
