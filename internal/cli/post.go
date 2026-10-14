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
	selection, err := a.selectionFrom(selectionFlags)
	if err != nil {
		return err
	}
	p, err := a.newPoster(*only, selection)
	if err != nil {
		return err
	}

	if *dryRun {
		out, err := a.prepare(p)
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(a, out.question)
		}
		for i, t := range p.targets {
			fmt.Fprintf(a.stdout, "would post to %s:\n%s\n\n", t.name, out.messages[i])
		}
		return nil
	}

	out, posted, err := a.pickAndSend(p)
	if out == nil {
		return err
	}
	// the question went out somewhere, so report it even if a target failed
	if posted > 0 && *asJSON {
		if err := printJSON(a, out.question); err != nil {
			return err
		}
	}
	return err
}

// poster is everything needed to post, built once from the config so
// a mistake stops cuppa before anything is picked or sent. The daemon
// keeps one for its whole run.
type poster struct {
	targets   []target
	renderer  *renderer
	selection config.Selection
	dataset   *cuppa.Dataset
	history   *history.File
}

func (a *app) newPoster(only string, selection config.Selection) (*poster, error) {
	targets, err := a.buildTargets(only)
	if err != nil {
		return nil, err
	}
	r, err := newRenderer(a.cfg.Message)
	if err != nil {
		return nil, err
	}
	dataset, err := a.questionsFor(selection)
	if err != nil {
		return nil, err
	}
	return &poster{
		targets:   targets,
		renderer:  r,
		selection: selection,
		dataset:   dataset,
		history:   history.NewFile(a.cfg.History.Path),
	}, nil
}

// outgoing is a picked question with its message for each target,
// ready to send.
type outgoing struct {
	question   cuppa.Question
	messages   []string // same order as poster.targets
	scratch    *cuppa.MemoryHistory
	usedBefore int
}

// prepare picks a question against a copy of the history and renders
// it for every target. Nothing is saved or sent yet.
func (a *app) prepare(p *poster) (*outgoing, error) {
	scratch, usedBefore, err := copyHistory(p.history)
	if err != nil {
		return nil, err
	}
	q, err := a.pick(p.dataset, p.selection, scratch)
	if err != nil {
		return nil, err
	}

	// each chat app writes bold its own way, so render per target,
	// all before sending anything
	messages := make([]string, len(p.targets))
	for i, t := range p.targets {
		if messages[i], err = p.renderer.render(q, t.bold); err != nil {
			return nil, err
		}
	}
	return &outgoing{question: q, messages: messages, scratch: scratch, usedBefore: usedBefore}, nil
}

// pickAndSend picks a question, posts it and saves the history. It
// holds a lock the whole time, so in "cuppa serve" two posts (or a post
// and /v1/questions/next) never pick the same question. The lock only
// covers this process; the history file has its own lock for others.
func (a *app) pickAndSend(p *poster) (*outgoing, int, error) {
	a.historyMu.Lock()
	defer a.historyMu.Unlock()

	out, err := a.prepare(p)
	if err != nil {
		return nil, 0, err
	}
	posted, err := a.send(p, out)
	return out, posted, err
}

// send posts to every target, even if one fails, and saves the
// history once at least one target got it. It returns how many
// targets got it, and an exit-code-3 error if any failed.
func (a *app) send(p *poster, out *outgoing) (int, error) {
	posted := 0
	for i, t := range p.targets {
		if err := t.adapter.Post(a.ctx, adapters.Message{Question: out.question, Rendered: out.messages[i]}); err != nil {
			a.log.Error("post failed", "target", t.name, "error", err)
			continue
		}
		a.log.Info("posted", "target", t.name, "question", out.question.ID)
		posted++
	}

	if posted > 0 && p.selection.Mode == config.ModeNoRepeat {
		if err := a.saveUsed(p.history, out.scratch, out.usedBefore, out.question.ID); err != nil {
			return posted, err
		}
	}
	if posted < len(p.targets) {
		return posted, &codedError{code: exitPostFailed, err: fmt.Errorf("posted to %d of %d targets", posted, len(p.targets))}
	}
	return posted, nil
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
