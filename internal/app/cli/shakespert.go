package cli

import (
	"context"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/urfave/cli/v2"

	"prospero/internal/features/shakespert"
)

var shakespertCmd = &cli.Command{
	Name:        "shakespert",
	Usage:       "Access Shakespeare's complete works",
	Description: `Access William Shakespeare's complete works including plays, poems, and sonnets.`,
	Subcommands: []*cli.Command{
		{
			Name:        "works",
			Usage:       "List all Shakespeare works",
			Description: `List all of Shakespeare's works with basic information.`,
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:    "genre",
					Aliases: []string{"g"},
					Usage:   "Filter works by genre (c=Comedy, h=History, p=Poem, s=Sonnet, t=Tragedy)",
				},
			},
			Action: func(c *cli.Context) error {
				genre := c.String("genre")
				return listWorks(c.Context, genre)
			},
		},
		{
			Name:        "work",
			Usage:       "Show details about a specific work",
			ArgsUsage:   "[workID]",
			Description: `Show detailed information about a specific Shakespeare work by ID.`,
			Action: func(c *cli.Context) error {
				if c.NArg() != 1 {
					return fmt.Errorf("exactly one workID argument is required")
				}
				return showWork(c.Context, c.Args().Get(0))
			},
		},
		{
			Name:        "genres",
			Usage:       "List all genres",
			Description: `List all available genres in the Shakespeare collection.`,
			Action: func(c *cli.Context) error {
				return listGenres(c.Context)
			},
		},
	},
}

func listWorks(ctx context.Context, genre string) error {
	service, err := shakespert.NewService(ctx)
	if err != nil {
		return fmt.Errorf("failed to initialize shakespert service: %w", err)
	}
	defer service.Close()

	var works []shakespert.WorkSummary

	if genre != "" {
		works, err = service.GetWorksByGenre(ctx, genre)
		if err != nil {
			return fmt.Errorf("failed to get works by genre: %w", err)
		}
	} else {
		works, err = service.ListWorks(ctx)
		if err != nil {
			return fmt.Errorf("failed to list works: %w", err)
		}
	}

	if len(works) == 0 {
		fmt.Println("No works found.")
		return nil
	}

	// Create tabwriter for formatted output
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintf(w, "ID\tTITLE\tGENRE\tYEAR\tWORDS\tPARAGRAPHS\n")
	fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
		strings.Repeat("-", 15),
		strings.Repeat("-", 30),
		strings.Repeat("-", 10),
		strings.Repeat("-", 6),
		strings.Repeat("-", 8),
		strings.Repeat("-", 12))

	for _, work := range works {
		year := ""
		if work.Date > 0 {
			year = fmt.Sprintf("%d", work.Date)
		}

		fmt.Fprintf(w, "%s\t%s\t%s (%s)\t%s\t%d\t%d\n",
			work.WorkID,
			truncateString(work.Title, 30),
			work.GenreName,
			work.GenreType,
			year,
			work.TotalWords,
			work.TotalParagraphs,
		)
	}

	return w.Flush()
}

func showWork(ctx context.Context, workID string) error {
	service, err := shakespert.NewService(ctx)
	if err != nil {
		return fmt.Errorf("failed to initialize shakespert service: %w", err)
	}
	defer service.Close()

	work, err := service.GetWork(ctx, workID)
	if err != nil {
		return fmt.Errorf("failed to get work: %w", err)
	}

	fmt.Printf("╭─ %s ─╮\n", strings.Repeat("─", len(work.Title)+2))
	fmt.Printf("│ %s │\n", work.Title)
	fmt.Printf("╰─%s─╯\n", strings.Repeat("─", len(work.Title)+2))
	fmt.Printf("\n")

	if work.LongTitle != work.Title && work.LongTitle != "" {
		fmt.Printf("Full Title: %s\n", work.LongTitle)
	}

	if work.ShortTitle != "" {
		fmt.Printf("Short Title: %s\n", work.ShortTitle)
	}

	fmt.Printf("Work ID: %s\n", work.WorkID)
	fmt.Printf("Genre: %s (%s)\n", work.GenreName, work.GenreType)

	if work.Date > 0 {
		fmt.Printf("Year: %d\n", work.Date)
	}

	fmt.Printf("Words: %d\n", work.TotalWords)
	fmt.Printf("Paragraphs: %d\n", work.TotalParagraphs)

	if work.Source != "" {
		fmt.Printf("Source: %s\n", work.Source)
	}

	if work.Notes != "" && work.Notes != "null" {
		fmt.Printf("Notes: %s\n", work.Notes)
	}

	return nil
}

func listGenres(ctx context.Context) error {
	service, err := shakespert.NewService(ctx)
	if err != nil {
		return fmt.Errorf("failed to initialize shakespert service: %w", err)
	}
	defer service.Close()

	genres, err := service.ListGenres(ctx)
	if err != nil {
		return fmt.Errorf("failed to list genres: %w", err)
	}

	fmt.Println("Available Genres:")
	fmt.Println("─────────────────")

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintf(w, "CODE\tNAME\n")
	fmt.Fprintf(w, "%s\t%s\n", strings.Repeat("-", 4), strings.Repeat("-", 20))

	for _, genre := range genres {
		genreName := genre.Genrename.String
		if !genre.Genrename.Valid {
			genreName = ""
		}
		fmt.Fprintf(w, "%s\t%s\n", genre.Genretype, genreName)
	}

	return w.Flush()
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
