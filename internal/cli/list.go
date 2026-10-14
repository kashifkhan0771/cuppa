package cli

import (
	"encoding/json"
	"fmt"
	"text/tabwriter"
)

// runList prints the questions the config's selection allows.
func runList(a *app, args []string) error {
	fs := a.newFlagSet("list", "list [--category a,b] [--json]")
	categories := fs.String("category", "", "only these categories, comma separated")
	asJSON := fs.Bool("json", false, "print as JSON")
	if err := a.parse(fs, args); err != nil {
		return err
	}
	if err := a.loadConfig(); err != nil {
		return err
	}

	selection, err := a.selection(*categories, "", "")
	if err != nil {
		return err
	}
	dataset, err := a.questionsFor(selection)
	if err != nil {
		return err
	}

	if *asJSON {
		return printJSON(a, dataset.All())
	}
	w := tabwriter.NewWriter(a.stdout, 0, 4, 2, ' ', 0)
	for _, q := range dataset.All() {
		fmt.Fprintf(w, "%s\t%s\t%s\n", q.ID, q.Category, q.Text)
	}
	return w.Flush()
}

// runCategories prints every category with its question count.
func runCategories(a *app, args []string) error {
	fs := a.newFlagSet("categories", "categories [--json]")
	asJSON := fs.Bool("json", false, "print as JSON")
	if err := a.parse(fs, args); err != nil {
		return err
	}
	if err := a.loadConfig(); err != nil {
		return err
	}

	dataset, err := a.allQuestions()
	if err != nil {
		return err
	}

	if *asJSON {
		return printJSON(a, dataset.Categories())
	}
	w := tabwriter.NewWriter(a.stdout, 0, 4, 2, ' ', 0)
	for _, c := range dataset.Categories() {
		fmt.Fprintf(w, "%s\t%s\t%d questions\n", c.ID, c.Name, c.Count)
	}
	return w.Flush()
}

func printJSON(a *app, v any) error {
	enc := json.NewEncoder(a.stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
