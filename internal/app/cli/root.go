package cli

import (
	"fmt"
	"os"

	"github.com/urfave/cli/v2"
)

var app = &cli.App{
	Name:  "prospero",
	Usage: "A magical box of edge services and fun utilities",
	Description: `Prospero is a single binary that can run as both a CLI tool and server.

It provides fun utilities like David Letterman Top 10 lists and serves them
via both HTTP and SSH interfaces when running in server mode.

Perfect for deployment on edge platforms like bunny.net Magic Containers.`,
	Commands: []*cli.Command{
		topTenCmd,
		shakespertCmd,
		serveCmd,
		devCmd,
		mcpCmd,
	},
}

// Execute runs the CLI application
func Execute() {
	if err := app.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
