package main

import (
	"github.com/alecthomas/kong"
	"github.com/palm-arcade/votigo/cmd"
)

func main() {
	var cli cmd.CLI
	ctx := kong.Parse(&cli,
		kong.Name("votigo"),
		kong.Description("Voting app for Palm's Arcade Retro LAN"),
		kong.UsageOnError(),
	)
	err := ctx.Run(&cli)
	ctx.FatalIfErrorf(err)
}
