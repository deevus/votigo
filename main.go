package main

import (
	"github.com/alecthomas/kong"
	"github.com/palm-arcade/votigo/cmd"
)

func main() {
	var cli cmd.CLI
	cmdCtx := &cmd.Context{}
	ctx := kong.Parse(&cli,
		kong.Name("votigo"),
		kong.Description("Voting app for Palm's Arcade Retro LAN"),
		kong.UsageOnError(),
		kong.Bind(cmdCtx),
	)
	err := ctx.Run(cmdCtx)
	ctx.FatalIfErrorf(err)
}
