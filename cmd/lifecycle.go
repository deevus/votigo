// cmd/lifecycle.go
package cmd

import (
	"context"
	"fmt"

	"github.com/palm-arcade/votigo/internal/db"
)

func (c *OpenCmd) Run(ctx *Context) error {
	// Check category exists
	cat, err := ctx.Queries.GetCategory(context.Background(), c.CategoryID)
	if err != nil {
		return fmt.Errorf("category not found: %w", err)
	}

	// Check has options
	count, err := ctx.Queries.CountOptionsByCategory(context.Background(), c.CategoryID)
	if err != nil {
		return err
	}
	if count == 0 {
		return fmt.Errorf("cannot open category with no options")
	}

	err = ctx.Queries.UpdateCategoryStatus(context.Background(), db.UpdateCategoryStatusParams{
		Status: "open",
		ID:     c.CategoryID,
	})
	if err != nil {
		return err
	}

	fmt.Printf("Opened voting for: %s\n", cat.Name)
	return nil
}

func (c *CloseCmd) Run(ctx *Context) error {
	cat, err := ctx.Queries.GetCategory(context.Background(), c.CategoryID)
	if err != nil {
		return fmt.Errorf("category not found: %w", err)
	}

	err = ctx.Queries.UpdateCategoryStatus(context.Background(), db.UpdateCategoryStatusParams{
		Status: "closed",
		ID:     c.CategoryID,
	})
	if err != nil {
		return err
	}

	fmt.Printf("Closed voting for: %s\n", cat.Name)
	return nil
}
