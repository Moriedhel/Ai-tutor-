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

const (
	checkTimeout  = 3 * time.Minute
	maxCheckBytes = 20_000 // output beyond this is cut off before it goes to Claude
)

// checks are the only commands RunCheck can run. Claude picks one by name and can't add arguments.
var checks = map[string][]string{
	"build": {"go", "build", "-o", os.DevNull, "./..."}, // compile everything, but don't leave a program file behind
	"vet":   {"go", "vet", "./..."},
	"test":  {"go", "test", "./..."},
}

// RunCheck lets Claude build, vet or test the Go code in the files folder, so it can check its own changes.
type RunCheck struct{ Dir FilesDir }

func (RunCheck) Definition() anthropic.ToolParam {
	return anthropic.ToolParam{
		Name: "run_check",
		Description: anthropic.String("Check the Go code in the user's files folder. " +
			"\"build\" compiles everything, \"vet\" looks for common mistakes, \"test\" runs the tests. " +
			"Run it after changing code, and fix any errors it reports. Returns OK or FAILED with the output."),
		Strict: anthropic.Bool(true),
		InputSchema: anthropic.ToolInputSchemaParam{
			Properties: map[string]any{
				"check": map[string]any{"type": "string", "enum": []string{"build", "vet", "test"}},
			},
			Required:    []string{"check"},
			ExtraFields: map[string]any{"additionalProperties": false},
		},
	}
}

func (r RunCheck) Run(ctx context.Context, raw json.RawMessage) (string, error) {
	var in struct {
		Check string `json:"check"`
	}
	if err := json.Unmarshal(raw, &in); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}
	args, ok := checks[in.Check]
	if !ok {
		return "", fmt.Errorf("unknown check %q", in.Check)
	}

	ctx, cancel := context.WithTimeout(ctx, checkTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Dir = r.Dir.osPath
	cmd.Env = withoutSecrets(os.Environ())

	fmt.Printf("\n(running %s)\n", strings.Join(args, " "))
	out, err := cmd.CombinedOutput()
	output := string(out)
	if len(output) > maxCheckBytes {
		output = output[:maxCheckBytes] + fmt.Sprintf("\n[cut off: showing %d of %d bytes]", maxCheckBytes, len(out))
	}

	var exitErr *exec.ExitError
	switch {
	case ctx.Err() == context.DeadlineExceeded:
		return fmt.Sprintf("FAILED: gave up after %s.\n%s", checkTimeout, output), nil
	case errors.As(err, &exitErr):
		fmt.Println("(check failed)")
		return "FAILED:\n" + output, nil
	case err != nil:
		return "", err // e.g. Go isn't installed
	}
	fmt.Println("(check passed)")
	if strings.TrimSpace(output) == "" {
		return "OK: no problems found.", nil
	}
	return "OK:\n" + output, nil
}

// withoutSecrets removes the API key from the environment, so code being tested can't read it.
func withoutSecrets(env []string) []string {
	var kept []string
	for _, kv := range env {
		if !strings.HasPrefix(kv, "ANTHROPIC_API_KEY=") {
			kept = append(kept, kv)
		}
	}
	return kept
}
