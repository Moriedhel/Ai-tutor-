package tutor

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadProfileMissingFile(t *testing.T) {
	p, err := LoadProfile(filepath.Join(t.TempDir(), "none.json"))
	if p != nil || err != nil {
		t.Fatalf("got %v, %v; want nil, nil", p, err)
	}
}

func TestCompleteProfile(t *testing.T) {
	p := &Profile{Name: "Ada", Subject: "Math", Audience: "kids", TeachingStyle: "games", LessonIdeas: []string{"fractions pizza"}}
	if len(p.Missing()) != 0 || !strings.Contains(p.String(), "Design complete.") {
		t.Fatalf("expected a complete design, got:\n%s", p)
	}
}
