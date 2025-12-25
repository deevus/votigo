package cmd

type CLI struct {
	DB string `help:"Path to database file" default:"votigo.db" type:"path"`

	Serve    ServeCmd    `cmd:"" help:"Start the web server"`
	Category CategoryCmd `cmd:"" help:"Manage voting categories"`
	Option   OptionCmd   `cmd:"" help:"Manage category options"`
	Open     OpenCmd     `cmd:"" help:"Open voting for a category"`
	Close    CloseCmd    `cmd:"" help:"Close voting for a category"`
	Results  ResultsCmd  `cmd:"" help:"Show results for a category"`
}

// Placeholder commands - will be implemented in later tasks
type ServeCmd struct {
	Port          int    `help:"Port to listen on" default:"5000"`
	AdminPassword string `help:"Password for admin interface" required:""`
}

type CategoryCmd struct {
	List   CategoryListCmd   `cmd:"" help:"List all categories"`
	Create CategoryCreateCmd `cmd:"" help:"Create a new category"`
}

type CategoryListCmd struct{}
type CategoryCreateCmd struct {
	Name    string `arg:"" help:"Category name"`
	Type    string `help:"Vote type: single, ranked, approval" default:"single" enum:"single,ranked,approval"`
	MaxRank int    `help:"Max rank for ranked voting" default:"3"`
}

type OptionCmd struct {
	Add    OptionAddCmd    `cmd:"" help:"Add option to category"`
	List   OptionListCmd   `cmd:"" help:"List options in category"`
	Remove OptionRemoveCmd `cmd:"" help:"Remove an option"`
}

type OptionAddCmd struct {
	CategoryID int64  `arg:"" help:"Category ID"`
	Name       string `arg:"" help:"Option name"`
}
type OptionListCmd struct {
	CategoryID int64 `arg:"" help:"Category ID"`
}
type OptionRemoveCmd struct {
	OptionID int64 `arg:"" help:"Option ID"`
}

type OpenCmd struct {
	CategoryID int64 `arg:"" help:"Category ID to open"`
}

type CloseCmd struct {
	CategoryID int64 `arg:"" help:"Category ID to close"`
}

type ResultsCmd struct {
	CategoryID int64 `arg:"" help:"Category ID"`
	ShowVoters bool  `help:"Show voter nicknames"`
}
