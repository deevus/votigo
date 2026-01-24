package cmd

import (
	"database/sql"

	"github.com/palm-arcade/votigo/internal/db"
)

// Context passed to all commands
type Context struct {
	DB      *sql.DB
	Queries *db.Queries
}

type CLI struct {
	DB string `help:"Path to database file" default:"votigo.db" type:"path"`

	Serve   ServeCmd   `cmd:"" help:"Start the web server"`
	Poll    PollCmd    `cmd:"" help:"Manage voting polls"`
	Option  OptionCmd  `cmd:"" help:"Manage poll options"`
	Open    OpenCmd    `cmd:"" help:"Open voting for a poll"`
	Close   CloseCmd   `cmd:"" help:"Close voting for a poll"`
	Reopen  ReopenCmd  `cmd:"" help:"Reopen voting for a closed poll"`
	Results ResultsCmd `cmd:"" help:"Show results for a poll"`
}

// Placeholder commands - will be implemented in later tasks
type ServeCmd struct {
	Port          int    `help:"Port to listen on" default:"5000"`
	AdminPassword string `help:"Password for admin interface" required:""`
	UI            string `help:"UI style" enum:"modern,legacy" default:"modern"`
}

type PollCmd struct {
	List   PollListCmd   `cmd:"" help:"List all polls"`
	Create PollCreateCmd `cmd:"" help:"Create a new poll"`
}

type PollListCmd struct{}
type PollCreateCmd struct {
	Name    string `arg:"" help:"Poll name"`
	Type    string `help:"Vote type: single, ranked, approval" default:"single" enum:"single,ranked,approval"`
	MaxRank int    `help:"Max rank for ranked voting" default:"3"`
}

type OptionCmd struct {
	Add    OptionAddCmd    `cmd:"" help:"Add option to poll"`
	List   OptionListCmd   `cmd:"" help:"List options in poll"`
	Remove OptionRemoveCmd `cmd:"" help:"Remove an option"`
}

type OptionAddCmd struct {
	CategoryID int64  `arg:"" help:"Poll ID"`
	Name       string `arg:"" help:"Option name"`
}
type OptionListCmd struct {
	CategoryID int64 `arg:"" help:"Poll ID"`
}
type OptionRemoveCmd struct {
	OptionID int64 `arg:"" help:"Option ID"`
}

type OpenCmd struct {
	CategoryID int64 `arg:"" help:"Poll ID to open"`
}

type CloseCmd struct {
	CategoryID int64 `arg:"" help:"Poll ID to close"`
}

type ReopenCmd struct {
	CategoryID int64 `arg:"" help:"Poll ID to reopen"`
}

type ResultsCmd struct {
	CategoryID int64 `arg:"" help:"Poll ID"`
	ShowVoters bool  `help:"Show voter nicknames"`
}

// AfterApply opens database connection
func (c *CLI) AfterApply(ctx *Context) error {
	conn, err := db.Open(c.DB)
	if err != nil {
		return err
	}

	if err := db.Migrate(conn); err != nil {
		conn.Close()
		return err
	}

	ctx.DB = conn
	ctx.Queries = db.New(conn)
	return nil
}
