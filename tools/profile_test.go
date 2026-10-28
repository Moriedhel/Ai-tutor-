package tools

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"tutor-agent/tutor"
)

func TestSaveToolRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "p.json")
	out, err := SaveProfile{Path: path}.Run(context.Background(), json.RawMessage(
		`{"name":"","subject":"Math","audience":"10-year-olds","teaching_style":"","lesson_ideas":[],"notes":""}`))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "teaching_style, lesson_ideas, name") {
		t.Fatalf("tool result should list undecided steps in order, got %q", out)
	}

	p, err := tutor.LoadProfile(path)
	if err != nil || p == nil || p.Subject != "Math" || p.Audience != "10-year-olds" {
		t.Fatalf("got %+v, %v", p, err)
	}
	if got := p.Missing(); len(got) != 3 {
		t.Fatalf("missing = %v, want 3 steps", got)
	}
}
