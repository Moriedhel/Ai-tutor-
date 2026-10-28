package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go/v2"

	"tutor-agent/tutor"
)

// SaveProfile writes the tutor's design to a JSON file, so the design can be resumed later.
type SaveProfile struct {
	Path string
}

func (SaveProfile) Definition() anthropic.ToolParam {
	return anthropic.ToolParam{
		Name: "save_tutor_profile",
		Description: anthropic.String("Save the tutor design to disk. Call it after every decision the user makes, " +
			"so their progress survives if they quit. Each call replaces the whole saved profile, so always send " +
			"every field: keep earlier decisions, and use \"\" (or [] for lesson_ideas) for anything still undecided. " +
			"Returns which steps are still undecided."),
		Strict: anthropic.Bool(true),
		InputSchema: anthropic.ToolInputSchemaParam{
			Properties: map[string]any{
				"name":           map[string]any{"type": "string", "description": "The tutor's name."},
				"subject":        map[string]any{"type": "string"},
				"audience":       map[string]any{"type": "string", "description": "Who the tutor teaches, e.g. age or level."},
				"teaching_style": map[string]any{"type": "string"},
				"lesson_ideas":   map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
				"notes":          map[string]any{"type": "string", "description": "Anything else the user wants the tutor to know or do."},
			},
			Required:    []string{"name", "subject", "audience", "teaching_style", "lesson_ideas", "notes"},
			ExtraFields: map[string]any{"additionalProperties": false},
		},
	}
}

func (s SaveProfile) Run(_ context.Context, raw json.RawMessage) (string, error) {
	var profile tutor.Profile
	if err := json.Unmarshal(raw, &profile); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}
	if err := profile.Save(s.Path); err != nil {
		return "", err
	}

	missing := profile.Missing()
	fmt.Printf("\n(saved tutor profile to %s, %d of 5 steps decided)\n", s.Path, 5-len(missing))
	if len(missing) == 0 {
		return "Saved. The design is complete.", nil
	}
	return "Saved. Still undecided: " + strings.Join(missing, ", ") + ".", nil
}
