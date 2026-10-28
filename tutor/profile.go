// Package tutor holds the tutor being designed: its profile, and saving and loading it.
package tutor

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
)

// Profile is the tutor being designed. Empty fields are still undecided.
type Profile struct {
	Name          string   `json:"name"`
	Subject       string   `json:"subject"`
	Audience      string   `json:"audience"`
	TeachingStyle string   `json:"teaching_style"`
	LessonIdeas   []string `json:"lesson_ideas"`
	Notes         string   `json:"notes"`
}

// LoadProfile reads a saved profile. It returns nil, nil when there is none yet.
func LoadProfile(path string) (*Profile, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var p Profile
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("%s is not a valid profile: %w", path, err)
	}
	return &p, nil
}

// Save writes the profile as JSON, replacing any file at path.
func (p *Profile) Save(path string) error {
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// Missing lists the design steps that are still undecided, in the order the design flow covers them.
func (p *Profile) Missing() []string {
	var m []string
	if p.Subject == "" {
		m = append(m, "subject")
	}
	if p.Audience == "" {
		m = append(m, "audience")
	}
	if p.TeachingStyle == "" {
		m = append(m, "teaching_style")
	}
	if len(p.LessonIdeas) == 0 {
		m = append(m, "lesson_ideas")
	}
	if p.Name == "" {
		m = append(m, "name")
	}
	return m
}

// String is the summary shown by /profile and given to Claude when a design is resumed.
func (p *Profile) String() string {
	orDash := func(s string) string {
		if s == "" {
			return "(not decided)"
		}
		return s
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Name:           %s\n", orDash(p.Name))
	fmt.Fprintf(&b, "Subject:        %s\n", orDash(p.Subject))
	fmt.Fprintf(&b, "Audience:       %s\n", orDash(p.Audience))
	fmt.Fprintf(&b, "Teaching style: %s\n", orDash(p.TeachingStyle))
	b.WriteString("Lesson ideas:\n")
	if len(p.LessonIdeas) == 0 {
		b.WriteString("  (not decided)\n")
	}
	for _, idea := range p.LessonIdeas {
		fmt.Fprintf(&b, "  - %s\n", idea)
	}
	if p.Notes != "" {
		fmt.Fprintf(&b, "Notes:          %s\n", p.Notes)
	}
	if m := p.Missing(); len(m) > 0 {
		fmt.Fprintf(&b, "Still to decide: %s\n", strings.Join(m, ", "))
	} else {
		b.WriteString("Design complete.\n")
	}
	return b.String()
}
