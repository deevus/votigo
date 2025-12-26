// cmd/option.go
package cmd

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/palm-arcade/votigo/internal/db"
)

func (c *OptionAddCmd) Run(ctx *Context) error {
	// Verify poll exists
	cat, err := ctx.Queries.GetCategory(context.Background(), c.CategoryID)
	if err != nil {
		return fmt.Errorf("poll not found: %w", err)
	}

	// Get current count for sort_order
	count, err := ctx.Queries.CountOptionsByCategory(context.Background(), c.CategoryID)
	if err != nil {
		return err
	}

	opt, err := ctx.Queries.CreateOption(context.Background(), db.CreateOptionParams{
		CategoryID: c.CategoryID,
		Name:       c.Name,
		SortOrder:  sql.NullInt64{Int64: count, Valid: true},
	})
	if err != nil {
		return err
	}

	fmt.Printf("Added option #%d to %s: %s\n", opt.ID, cat.Name, opt.Name)
	return nil
}

func (c *OptionListCmd) Run(ctx *Context) error {
	cat, err := ctx.Queries.GetCategory(context.Background(), c.CategoryID)
	if err != nil {
		return fmt.Errorf("poll not found: %w", err)
	}

	options, err := ctx.Queries.ListOptionsByCategory(context.Background(), c.CategoryID)
	if err != nil {
		return err
	}

	fmt.Printf("Options for: %s\n\n", cat.Name)

	if len(options) == 0 {
		fmt.Println("No options yet.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tNAME")
	for _, opt := range options {
		fmt.Fprintf(w, "%d\t%s\n", opt.ID, opt.Name)
	}
	w.Flush()

	return nil
}

func (c *OptionRemoveCmd) Run(ctx *Context) error {
	opt, err := ctx.Queries.GetOption(context.Background(), c.OptionID)
	if err != nil {
		return fmt.Errorf("option not found: %w", err)
	}

	err = ctx.Queries.DeleteOption(context.Background(), c.OptionID)
	if err != nil {
		return err
	}

	fmt.Printf("Removed option: %s\n", opt.Name)
	return nil
}
