package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/urfave/cli/v2"

	"prospero/assets"
	"prospero/internal/mcp"
)

var mcpCmd = &cli.Command{
	Name:        "mcp",
	Usage:       "Start the MCP (Model Context Protocol) server",
	Description: `Start the MCP server with stdio transport. The server exposes prompts defined in assets/prompts/*.toml files.`,
	Action: func(c *cli.Context) error {
		ctx := c.Context

		// Create MCP server
		server := mcp.NewServer("prospero", "1.0.0")

		// Load prompts from TOML files
		promptFS := assets.GetEmbeddedPrompts()
		definitions, err := mcp.LoadPromptsFromTOML(promptFS)
		if err != nil {
			return fmt.Errorf("failed to load prompts: %w", err)
		}

		// Register prompts with placeholder handlers
		for _, def := range definitions {
			prompt := def.ToPrompt()
			// Register with a placeholder handler that returns an error
			// Users will need to implement actual handlers
			server.RegisterPrompt(prompt, func(ctx context.Context, args map[string]string) (*mcp.GetPromptResult, error) {
				return nil, fmt.Errorf("handler not implemented for prompt: %s", prompt.Name)
			})
		}

		// Log loaded prompts to stderr
		fmt.Fprintf(os.Stderr, "Loaded %d prompts:\n", len(definitions))
		for _, def := range definitions {
			fmt.Fprintf(os.Stderr, "  - %s: %s\n", def.Name, def.Description)
		}

		// Start the server
		fmt.Fprintln(os.Stderr, "MCP server starting on stdio...")
		return server.Run(ctx)
	},
}
