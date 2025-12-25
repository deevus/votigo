// cmd/category.go
package cmd

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"
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
