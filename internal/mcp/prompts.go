package mcp

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

type PromptHandler func(ctx context.Context, args map[string]string) (*GetPromptResult, error)

type PromptRegistry struct {
	prompts  map[string]Prompt
	handlers map[string]PromptHandler
}

type PromptDefinition struct {
	Name        string               `toml:"name"`
	Description string               `toml:"description"`
	Arguments   []ArgumentDefinition `toml:"arguments"`
	Content     string               // Markdown content for the prompt
}

type ArgumentDefinition struct {
	Name        string `toml:"name"`
	Description string `toml:"description"`
	Required    bool   `toml:"required"`
}

func NewPromptRegistry() *PromptRegistry {
	return &PromptRegistry{
		prompts:  make(map[string]Prompt),
		handlers: make(map[string]PromptHandler),
	}
}

func (r *PromptRegistry) Register(prompt Prompt, handler PromptHandler) {
	r.prompts[prompt.Name] = prompt
	r.handlers[prompt.Name] = handler
}

func (r *PromptRegistry) List() []Prompt {
	prompts := make([]Prompt, 0, len(r.prompts))
	for _, prompt := range r.prompts {
		prompts = append(prompts, prompt)
	}
	return prompts
}

func (r *PromptRegistry) Execute(ctx context.Context, name string, args map[string]string) (*GetPromptResult, error) {
	handler, exists := r.handlers[name]
	if !exists {
		return nil, fmt.Errorf("prompt not found: %s", name)
	}

	return handler(ctx, args)
}

func LoadPromptsFromTOML(promptFiles embed.FS) ([]PromptDefinition, error) {
	var definitions []PromptDefinition

	err := fs.WalkDir(promptFiles, "prompts", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		ext := filepath.Ext(path)
		if ext != ".toml" && ext != ".md" {
			return nil
		}

		data, err := promptFiles.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", path, err)
		}

		var def PromptDefinition

		if ext == ".md" {
			// Extract TOML frontmatter and content from markdown
			def, err = parseMarkdownPrompt(data)
			if err != nil {
				return fmt.Errorf("failed to parse %s: %w", path, err)
			}
		} else {
			// Parse TOML directly
			if err := toml.Unmarshal(data, &def); err != nil {
				return fmt.Errorf("failed to parse %s: %w", path, err)
			}
		}

		definitions = append(definitions, def)
		return nil
	})
	if err != nil {
		return nil, err
	}

	return definitions, nil
}

func parseMarkdownPrompt(data []byte) (PromptDefinition, error) {
	content := string(data)

	// Check for TOML frontmatter (between +++ delimiters)
	if !strings.HasPrefix(content, "+++") {
		return PromptDefinition{}, fmt.Errorf("markdown file must start with +++ frontmatter delimiter")
	}

	// Find the closing +++
	endIndex := strings.Index(content[3:], "+++")
	if endIndex == -1 {
		return PromptDefinition{}, fmt.Errorf("missing closing +++ delimiter for frontmatter")
	}
	endIndex += 3 // Adjust for the offset

	// Extract frontmatter and content
	frontmatter := content[3:endIndex]
	promptContent := strings.TrimSpace(content[endIndex+3:])

	// Parse frontmatter as TOML
	var def PromptDefinition
	if err := toml.Unmarshal([]byte(frontmatter), &def); err != nil {
		return PromptDefinition{}, fmt.Errorf("failed to parse frontmatter: %w", err)
	}

	def.Content = promptContent
	return def, nil
}

func (d *PromptDefinition) ToPrompt() Prompt {
	args := make([]PromptArgument, len(d.Arguments))
	for i, arg := range d.Arguments {
		args[i] = PromptArgument{
			Name:        arg.Name,
			Description: arg.Description,
			Required:    arg.Required,
		}
	}

	return Prompt{
		Name:        d.Name,
		Description: d.Description,
		Arguments:   args,
	}
}

// CreateHandler returns a PromptHandler that substitutes arguments in the content
func (d *PromptDefinition) CreateHandler() PromptHandler {
	return func(ctx context.Context, args map[string]string) (*GetPromptResult, error) {
		if d.Content == "" {
			return nil, fmt.Errorf("no content available for prompt: %s", d.Name)
		}

		// Substitute arguments in the content
		content := d.Content
		for name, value := range args {
			placeholder := fmt.Sprintf("{{%s}}", name)
			content = strings.ReplaceAll(content, placeholder, value)
		}

		return &GetPromptResult{
			Description: d.Description,
			Messages: []PromptMessage{
				{
					Role: "user",
					Content: MessageContent{
						Type: "text",
						Text: content,
					},
				},
			},
		}, nil
	}
}
