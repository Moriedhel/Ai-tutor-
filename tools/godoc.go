package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go/v2"
)

// GoDoc lets Claude look up Go documentation by running the `go doc` command.
// It's a small example of turning a command-line program into a tool.
type GoDoc struct{ Dir FilesDir }

func (GoDoc) Definition() anthropic.ToolParam {
	return anthropic.ToolParam{
		Name: "go_doc",
		Description: anthropic.String("Show the documentation for a Go package, function or type, " +
			"e.g. \"strings.Contains\", \"os.Root\" or \"tutor-agent/tutor.Profile\". " +
			"Use it to check how something works before writing code that uses it."),
		Strict: anthropic.Bool(true),
		InputSchema: anthropic.ToolInputSchemaParam{
			Properties: map[string]any{
				"symbol": map[string]any{"type": "string", "description": "What to look up, e.g. \"fmt.Println\"."},
			},
			Required:    []string{"symbol"},
			ExtraFields: map[string]any{"additionalProperties": false},
		},
	}
}

func (g GoDoc) Run(ctx context.Context, raw json.RawMessage) (string, error) {
	var in struct {
		Symbol string `json:"symbol"`
	}
	if err := json.Unmarshal(raw, &in); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}
	// A symbol never starts with "-"; refusing it means Claude can't sneak in an option.
	if in.Symbol == "" || strings.HasPrefix(in.Symbol, "-") {
		return "", fmt.Errorf("%q is not a Go symbol", in.Symbol)
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	// The program and its options are fixed here; only the symbol comes from Claude, as one argument.
	cmd := exec.CommandContext(ctx, "go", "doc", in.Symbol)
	cmd.Dir = g.Dir.osPath // so it can also find packages in the user's own module
	cmd.Env = withoutSecrets(os.Environ())

	fmt.Printf("\n(running go doc %s)\n", in.Symbol)
	out, err := cmd.CombinedOutput()
	var exitErr *exec.ExitError
	switch {
	case errors.As(err, &exitErr):
		// Not an error in the tool: Claude just asked for something that doesn't exist.
		return "No documentation found:\n" + string(out), nil
	case err != nil:
		return "", err // e.g. Go isn't installed
	}
	return string(out), nil
}
