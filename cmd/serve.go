// cmd/serve.go
package cmd

import (
	"github.com/palm-arcade/votigo/internal/web"
)

func (c *ServeCmd) Run(ctx *Context) error {
	server, err := web.NewServer(ctx.DB, c.AdminPassword)
	if err != nil {
		return err
	}

	return server.Start(c.Port)
}
