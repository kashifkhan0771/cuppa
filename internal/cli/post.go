package cli

import (
	"errors"
	"fmt"

	"github.com/kashifkhan0771/cuppa/internal/config"
	"github.com/kashifkhan0771/cuppa/internal/history"
	"github.com/kashifkhan0771/cuppa/pkg/adapters"
	"github.com/kashifkhan0771/cuppa/pkg/cuppa"
)

// runPost picks a question and posts it to every target (or one).
//
// The question is picked against a copy of the history, and the real
// history is only updated once at least one target got the message.
// A dry run never touches the history and never sends anything.
func runPost(a *app, args []string) error {
	fs := a.newFlagSet("post", "post [--target name] [--dry-run] [--json] [--category a,b] [--tag x,y] [--mode m]")
	only := fs.String("target", "", "post to this target only (default: all)")
	dryRun := fs.Bool("dry-run", false, "show what would be posted, without posting")
	asJSON := fs.Bool("json", false, "print the posted question as JSON")
	selectionFlags := addSelectionFlags(fs)
	if err := a.parse(fs, args); err != nil {
		return err
	}
	if err := a.loadConfig(); err != nil {
		return err
	}
	if a.cfg.Path == "" {
		return configError(errNoConfig)
	}

	// build everything first, so a config mistake stops us before
	// anything is picked or sent
	targets, err := a.buildTargets(*only)
	if err != nil {
		return err
	}
	r, err := newRenderer(a.cfg.Message)
	if err != nil {
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

	saved := history.NewFile(a.cfg.History.Path)
	scratch, usedBefore, err := copyHistory(saved)
	if err != nil {
		return err
	}
	q, err := a.pick(dataset, selection, scratch)
	if err != nil {
		return err
	}
	message, err := r.render(q)
	if err != nil {
		return err
	}

	if *dryRun {
		if *asJSON {
			return printJSON(a, q)
		}
		for _, t := range targets {
			fmt.Fprintf(a.stdout, "would post to %s:\n%s\n\n", t.name, message)
		}
		return nil
	}

	// post to every target, even if one fails
	posted := 0
	for _, t := range targets {
		if err := t.adapter.Post(a.ctx, adapters.Message{Question: q, Rendered: message}); err != nil {
			a.log.Error("post failed", "target", t.name, "error", err)
			continue
		}
		a.log.Info("posted", "target", t.name, "question", q.ID)
		posted++
	}

	if posted > 0 && selection.Mode == config.ModeNoRepeat {
		if err := a.saveUsed(saved, scratch, usedBefore, q.ID); err != nil {
			return err
		}
	}
	// the question went out somewhere, so report it even if a target failed
	if posted > 0 && *asJSON {
		if err := printJSON(a, q); err != nil {
			return err
		}
	}
	if posted < len(targets) {
		return &codedError{code: exitPostFailed, err: fmt.Errorf("posted to %d of %d targets", posted, len(targets))}
	}
	return nil
}

// copyHistory loads the saved history into memory, so picking a
// question doesn't save anything yet. It also returns how many
// questions were used before.
func copyHistory(saved *history.File) (*cuppa.MemoryHistory, int, error) {
	used, err := saved.Used()
	if err != nil {
		return nil, 0, err
	}
	scratch := &cuppa.MemoryHistory{}
	for id, at := range used {
		if err := scratch.MarkUsed(id, at); err != nil {
			return nil, 0, err
		}
	}
	return scratch, len(used), nil
}

// saveUsed writes the pick to the saved history. If picking started
// a new cycle (the copy shrank to just this question), the saved
// history is reset first, the same way.
func (a *app) saveUsed(saved *history.File, scratch *cuppa.MemoryHistory, usedBefore int, id string) error {
	usedAfter, err := scratch.Used()
	if err != nil {
		return err
	}
	if usedBefore > 0 && len(usedAfter) == 1 {
		if err := saved.Reset(); err != nil {
			return errors.Join(errors.New("posted, but could not reset history"), err)
		}
	}
	if err := saved.MarkUsed(id, a.now()); err != nil {
		return errors.Join(errors.New("posted, but could not save history"), err)
	}
	return nil
}
