package tools

import (
	"context"
	"strings"
	"testing"
)

func TestProposeOptionsAnswers(t *testing.T) {
	menu := []byte(`{"question":"Subject?","options":["Math","Go"]}`)
	var queued string
	run := func(answer string) string {
		p := ProposeOptions{In: answers(answer), OnCommand: func(c string) { queued = c }}
		out, err := p.Run(context.Background(), menu)
		if err != nil {
			t.Fatal(err)
		}
		return out
	}

	if out := run("2\n"); out != "The user picked: Go" {
		t.Fatalf("number: got %q", out)
	}
	if out := run("\n"); !strings.Contains(out, "skipped") {
		t.Fatalf("Enter should skip, got %q", out)
	}
	if out := run("/new\n"); queued != "/new" || !strings.Contains(out, "End your reply") {
		t.Fatalf("/new should be queued for the program, got queued=%q out=%q", queued, out)
	}
	if out := run("something else\n"); !strings.Contains(out, `"something else"`) {
		t.Fatalf("free text should reach Claude, got %q", out)
	}
}
