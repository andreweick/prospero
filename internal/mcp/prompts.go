package mcp

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"path/filepath"

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

		if d.IsDir() || filepath.Ext(path) != ".toml" {
			return nil
		}

		data, err := promptFiles.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", path, err)
		}

		var def PromptDefinition
		if err := toml.Unmarshal(data, &def); err != nil {
			return fmt.Errorf("failed to parse %s: %w", path, err)
		}

		definitions = append(definitions, def)
		return nil
	})
	if err != nil {
		return nil, err
	}

	return definitions, nil
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
