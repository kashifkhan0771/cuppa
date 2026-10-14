package cli

import (
	"fmt"

	"github.com/kashifkhan0771/cuppa/internal/history"
	"github.com/kashifkhan0771/cuppa/pkg/cuppa"
)

// runHistory shows or resets the no-repeat history.
func runHistory(a *app, args []string) error {
	fs := a.newFlagSet("history", "history [show|reset]")
	if err := a.parse(fs, args); err != nil {
		return err
	}
	if err := a.loadConfig(); err != nil {
		return err
	}
	h := history.NewFile(a.cfg.History.Path)

	switch action := fs.Arg(0); action {
	case "", "show":
		used, err := h.Used()
		if err != nil {
			return err
		}
		cycles, err := h.Cycles()
		if err != nil {
			return err
		}
		fmt.Fprintf(a.stdout, "history file: %s\nused this cycle: %d of %d built-in questions\nfinished cycles: %d\n",
			a.cfg.History.Path, len(used), cuppa.Embedded().Len(), cycles)
		return nil
	case "reset":
		if err := h.Reset(); err != nil {
			return err
		}
		fmt.Fprintln(a.stdout, "history reset")
		return nil
	default:
		return configError(fmt.Errorf("unknown history action %q: use show or reset", action))
	}
}

// runVersion prints the build and dataset versions.
func runVersion(a *app, args []string) error {
	fs := a.newFlagSet("version", "version")
	if err := a.parse(fs, args); err != nil {
		return err
	}

	fmt.Fprintf(a.stdout, "cuppa %s", Version)
	if Commit != "" {
		fmt.Fprintf(a.stdout, " (commit %s, built %s)", Commit, Date)
	}
	d := cuppa.Embedded()
	fmt.Fprintf(a.stdout, "\nquestions %s (%d built in)\n", d.Version(), d.Len())
	return nil
}
