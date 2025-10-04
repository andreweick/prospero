package cli

import (
	"fmt"
	"os"

	"github.com/urfave/cli/v2"

	"prospero/internal/features/dev"
)

var devCmd = &cli.Command{
	Name:  "dev",
	Usage: "Development utilities for managing embedded data",
	Description: `Development utilities for extracting, packing, and rotating embedded data files.

This command provides tools for:
- Extracting embedded encrypted and compressed data for local development
- Packing modified data back into embedded format
- Safely rotating encryption keys`,
	Subcommands: []*cli.Command{
		{
			Name:      "extract",
			Usage:     "Extract embedded data files for development",
			ArgsUsage: "[type]",
			Description: `Extract embedded data files for local development.

Types:
  all        - Extract all data files (default)
  secrets    - Extract all age-encrypted files (topten.json, hostkey)
  topten     - Extract topten.json from topten.json.age
  hostkey    - Extract SSH host key from hostkey.age
  shakespert - Extract shakespert.sql and shakespert.db from shakespert.sql.gz

Extracted files are placed in the current directory and are gitignored.`,
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:  "output-dir",
					Value: ".",
					Usage: "Output directory for extracted files",
				},
				&cli.BoolFlag{
					Name:  "force",
					Usage: "Overwrite existing files",
				},
			},
			Action: runExtract,
		},
		{
			Name:      "pack",
			Usage:     "Pack modified data back into embedded format",
			ArgsUsage: "<type>",
			Description: `Pack modified data back into embedded format for inclusion in builds.

Types:
  shakespert - Compress shakespert.db or shakespert.sql into shakespert.sql.gz

The pack command will automatically detect input files in the current directory
and output compressed files to assets/data/ for embedding.`,
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:  "input",
					Usage: "Input file (auto-detected if not specified)",
				},
				&cli.StringFlag{
					Name:  "output-dir",
					Value: "assets/data",
					Usage: "Output directory for packed files",
				},
				&cli.BoolFlag{
					Name:  "force",
					Usage: "Overwrite existing files",
				},
				&cli.IntFlag{
					Name:  "compression",
					Value: 9,
					Usage: "Gzip compression level (1-9)",
				},
			},
			Action: runPack,
		},
		{
			Name:  "rotate-key",
			Usage: "Safely rotate age encryption keys",
			Description: `Safely rotate age encryption keys for all encrypted embedded data.

This command requires both environment variables:
- AGE_ENCRYPTION_PASSWORD: The new password
- PREVIOUS_AGE_ENCRYPTION_PASSWORD: The old password

The rotation is atomic - either all files are rotated successfully or none are modified.

Process:
1. Decrypt all .age files with the previous password
2. Re-encrypt all files with the new password
3. Verify new encryption works
4. Write all files atomically

After successful rotation, you can remove PREVIOUS_AGE_ENCRYPTION_PASSWORD.`,
			Flags: []cli.Flag{
				&cli.BoolFlag{
					Name:  "dry-run",
					Usage: "Test rotation without modifying files",
				},
				&cli.BoolFlag{
					Name:  "backup",
					Usage: "Create backup files before rotation",
				},
			},
			Action: runRotateKey,
		},
	},
}

func runExtract(c *cli.Context) error {
	outputDir := c.String("output-dir")
	force := c.Bool("force")

	opts := dev.ExtractOptions{
		OutputDir: outputDir,
		Force:     force,
	}

	// Determine what to extract
	extractType := "all"
	if c.NArg() > 0 {
		extractType = c.Args().Get(0)
	}

	ctx := c.Context

	switch extractType {
	case "all":
		return dev.ExtractAll(ctx, opts)

	case "secrets":
		return dev.ExtractSecrets(ctx, opts)

	case "topten":
		return dev.ExtractTopTen(ctx, opts)

	case "hostkey":
		return dev.ExtractHostKey(ctx, opts)

	case "shakespert":
		return dev.ExtractShakespert(ctx, opts)

	default:
		return fmt.Errorf("unknown extract type: %s\nValid types: all, secrets, topten, hostkey, shakespert", extractType)
	}
}

func runPack(c *cli.Context) error {
	if c.NArg() != 1 {
		return fmt.Errorf("exactly one pack type argument is required")
	}
	packType := c.Args().Get(0)

	inputFile := c.String("input")
	outputDir := c.String("output-dir")
	force := c.Bool("force")
	compression := c.Int("compression")

	opts := dev.PackOptions{
		InputFile:   inputFile,
		OutputDir:   outputDir,
		Force:       force,
		Compression: compression,
	}

	switch packType {
	case "shakespert":
		return dev.PackShakespert(opts)

	default:
		return fmt.Errorf("unknown pack type: %s\nValid types: shakespert", packType)
	}
}

func runRotateKey(c *cli.Context) error {
	dryRun := c.Bool("dry-run")
	backup := c.Bool("backup")

	opts := dev.RotateKeyOptions{
		DryRun: dryRun,
		Backup: backup,
	}

	// Check that we're in the project root (has assets/data directory)
	if _, err := os.Stat("assets/data"); os.IsNotExist(err) {
		pwd, _ := os.Getwd()
		return fmt.Errorf("assets/data directory not found\nPlease run this command from the project root directory\nCurrent directory: %s", pwd)
	}

	return dev.RotateKeys(c.Context, opts)
}
