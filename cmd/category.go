// cmd/category.go
package cmd

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/palm-arcade/votigo/internal/db"
)

func (c *CategoryListCmd) Run(ctx *Context) error {
	categories, err := ctx.Queries.ListCategories(context.Background())
	if err != nil {
		return err
	}

	if len(categories) == 0 {
		fmt.Println("No categories found.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tNAME\tTYPE\tSTATUS")
	for _, cat := range categories {
		fmt.Fprintf(w, "%d\t%s\t%s\t%s\n", cat.ID, cat.Name, cat.VoteType, cat.Status)
	}
	w.Flush()

	return nil
}

func (c *CategoryCreateCmd) Run(ctx *Context) error {
	var maxRank sql.NullInt64
	if c.Type == "ranked" {
		maxRank = sql.NullInt64{Int64: int64(c.MaxRank), Valid: true}
	}

	cat, err := ctx.Queries.CreateCategory(context.Background(), db.CreateCategoryParams{
		Name:        c.Name,
		VoteType:    c.Type,
		Status:      "draft",
		ShowResults: "after_close",
		MaxRank:     maxRank,
	})
	if err != nil {
		return err
	}

	fmt.Printf("Created category #%d: %s (%s)\n", cat.ID, cat.Name, cat.VoteType)
	return nil
}
