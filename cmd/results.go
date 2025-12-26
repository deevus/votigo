// cmd/results.go
package cmd

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/palm-arcade/votigo/internal/db"
)

func (c *ResultsCmd) Run(ctx *Context) error {
	cat, err := ctx.Queries.GetCategory(context.Background(), c.CategoryID)
	if err != nil {
		return fmt.Errorf("poll not found: %w", err)
	}

	voteCount, err := ctx.Queries.CountVotesByCategory(context.Background(), c.CategoryID)
	if err != nil {
		return err
	}

	fmt.Printf("Results for: %s (%d votes)\n\n", cat.Name, voteCount)

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

	if cat.VoteType == "ranked" {
		maxRank := sql.NullInt64{Int64: 3, Valid: true}
		if cat.MaxRank.Valid {
			maxRank = cat.MaxRank
		}

		results, err := ctx.Queries.TallyRanked(context.Background(), db.TallyRankedParams{
			MaxRank:    maxRank,
			CategoryID: c.CategoryID,
		})
		if err != nil {
			return err
		}

		fmt.Fprintln(w, "RANK\tOPTION\tPOINTS\t1ST PLACE")
		for i, r := range results {
			// Points is interface{} due to COALESCE, convert to int64
			points := int64(0)
			if r.Points != nil {
				switch v := r.Points.(type) {
				case int64:
					points = v
				case float64:
					points = int64(v)
				}
			}
			fmt.Fprintf(w, "%d\t%s\t%d\t%d\n", i+1, r.Name, points, r.FirstPlaceVotes)
		}
	} else {
		results, err := ctx.Queries.TallySimple(context.Background(), c.CategoryID)
		if err != nil {
			return err
		}

		fmt.Fprintln(w, "RANK\tOPTION\tVOTES")
		for i, r := range results {
			fmt.Fprintf(w, "%d\t%s\t%d\n", i+1, r.Name, r.Votes)
		}
	}

	w.Flush()

	if c.ShowVoters {
		fmt.Println("\nVoters:")
		voters, err := ctx.Queries.ListVotersByCategory(context.Background(), c.CategoryID)
		if err != nil {
			return err
		}
		for _, v := range voters {
			fmt.Printf("  - %s\n", v)
		}
	}

	return nil
}
