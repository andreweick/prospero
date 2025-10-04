package cli

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/urfave/cli/v2"
	"golang.org/x/term"

	"prospero/internal/app/server"
)

var serveCmd = &cli.Command{
	Name:  "serve",
	Usage: "Start the Prospero server (HTTP + SSH)",
	Description: `Start the Prospero server with both HTTP and SSH interfaces.

SSH server is automatically disabled when running on bunny.net Magic Containers.
Use --force-ssh to override this behavior for testing.`,
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "host",
			Value: "localhost",
			Usage: "Host to bind both servers to",
		},
		&cli.StringFlag{
			Name:  "http-port",
			Value: "8080",
			Usage: "Port for the HTTP server",
		},
		&cli.StringFlag{
			Name:  "ssh-port",
			Value: "2222",
			Usage: "Port for the SSH server",
		},
		&cli.BoolFlag{
			Name:  "force-ssh",
			Value: false,
			Usage: "Force SSH server to start even on bunny.net Magic Containers",
		},
	},
	Action: func(c *cli.Context) error {
		host := c.String("host")
		httpPort := c.String("http-port")
		sshPort := c.String("ssh-port")
		forceSSH := c.Bool("force-ssh")

		config := server.ServerConfig{
			Host:     host,
			HTTPPort: httpPort,
			SSHPort:  sshPort,
			ForceSSH: forceSSH,
		}

		// Create a context that cancels on SIGINT or SIGTERM
		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer cancel()

		// Start keyboard input handler in a goroutine
		go func() {
			// Save original terminal state
			oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
			if err != nil {
				// If we can't set raw mode, just return silently
				return
			}
			defer term.Restore(int(os.Stdin.Fd()), oldState)

			// Read single bytes from stdin
			buf := make([]byte, 1)
			for {
				_, err := os.Stdin.Read(buf)
				if err != nil {
					return
				}

				// Check for 'q' or 'Q'
				if buf[0] == 'q' || buf[0] == 'Q' {
					cancel()
					return
				}
			}
		}()

		return server.StartServers(ctx, config)
	},
}
