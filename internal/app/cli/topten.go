package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/urfave/cli/v2"

	"prospero/internal/features/topten"
)

var topTenCmd = &cli.Command{
	Name:        "topten",
	Usage:       "Display a random David Letterman Top 10 list",
	Description: `Display a random David Letterman Top 10 list with colorful formatting.`,
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:  "ascii",
			Usage: "Display output using ASCII characters only (no colors)",
		},
	},
	Action: func(c *cli.Context) error {
		ascii := c.Bool("ascii")
		return showRandomList(c.Context, ascii)
	},
}

func showRandomList(ctx context.Context, ascii bool) error {
	// Set ASCII mode if requested
	if ascii {
		lipgloss.SetColorProfile(termenv.Ascii)
	}

	service, err := topten.NewService(ctx)
	if err != nil {
		return fmt.Errorf("failed to initialize service: %w", err)
	}

	list, err := service.GetRandomList()
	if err != nil {
		return fmt.Errorf("failed to get random list: %w", err)
	}

	if ascii {
		topten.PrintListASCII(os.Stdout, list)
	} else {
		topten.PrintList(os.Stdout, list)
	}
	return nil
}
