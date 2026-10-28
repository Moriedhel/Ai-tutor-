// Command tutor-agent is a terminal chat with Claude that helps you design a tutor.
//
// The harness lives in the agent package, the tools in tools, and the tutor profile in tutor.
// This file starts the program and handles the / commands; tools.go picks the tools.
package main

import (
	"bufio"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/anthropics/anthropic-sdk-go/v2"
	"github.com/joho/godotenv"

	"tutor-agent/agent"
	"tutor-agent/tools"
	"tutor-agent/tutor"
)

// Settings that are part of how the program works. Per-machine settings come from .env (see .env.example).
const (
	defaultModel = "claude-opus-5-5" // used when MODEL is not set
	maxTokens    = 64000
	profilePath  = "tutor_profile.json"
	backupPath   = "tutor_profile.bak.json"
)

// systemPrompt tells Claude who it is and how to use its tools. Edit system_prompt.md to change it.
//
//go:embed system_prompt.md
var systemPrompt string

func main() {
	if err := godotenv.Load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		fmt.Fprintln(os.Stderr, "load .env:", err)
		os.Exit(1)
	}

	if os.Getenv("ANTHROPIC_API_KEY") == "" {
		fmt.Fprintln(os.Stderr, "Set ANTHROPIC_API_KEY in .env or in your environment.")
		os.Exit(1)
	}

	files, err := tools.OpenFilesDir(envOr("FILES_DIR", ".")) // the folder the agent may read and write
	if err != nil {
		fmt.Fprintln(os.Stderr, "open FILES_DIR:", err)
		os.Exit(1)
	}

	in := bufio.NewScanner(os.Stdin)
	in.Buffer(make([]byte, 1024*1024), 1024*1024)

	// A /command typed at a menu waits here until Claude's reply ends, then runs as if typed at you>.
	var pending string
	queue := func(command string) { pending = command }

	designer := agent.New(anthropic.NewClient(),
		agent.Config{Model: envOr("MODEL", defaultModel), MaxTokens: maxTokens, System: systemPrompt},
		allTools(deps{in: in, files: files, onCommand: queue})...)
	ctx := context.Background()

	fmt.Print(`Tutor agent. Type a message, or one of these commands:
  /profile   see the design so far
  /new       start a new tutor
  /quit      exit
`)
	if err := startDesign(ctx, designer); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
	}
	for {
		text := pending
		pending = ""
		if text == "" {
			fmt.Print("\nyou> ")
			if !in.Scan() {
				return
			}
			text = strings.TrimSpace(in.Text())
		}
		switch text {
		case "":
			continue
		case "/quit", "/exit":
			return
		case "/profile":
			showProfile()
			continue
		case "/new":
			if err := startOver(ctx, designer, in); err != nil {
				fmt.Fprintln(os.Stderr, "error:", err)
			}
			continue
		}
		if err := designer.Turn(ctx, text); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
		}
	}
}

// startDesign opens the conversation: it resumes the saved design if there is one, or starts a new one.
func startDesign(ctx context.Context, designer *agent.Agent) error {
	profile, err := tutor.LoadProfile(profilePath)
	if err != nil {
		return err
	}
	if profile == nil {
		return designer.Turn(ctx, "Let's design a new tutor.")
	}

	fmt.Printf("\nResuming your saved tutor design:\n%s", profile)
	saved, err := json.MarshalIndent(profile, "", "  ")
	if err != nil {
		return err
	}
	return designer.Turn(ctx, "I'm resuming my tutor design. This is what's saved so far:\n\n"+
		string(saved)+"\n\nPick up from the first undecided step.")
}

func showProfile() {
	profile, err := tutor.LoadProfile(profilePath)
	switch {
	case err != nil:
		fmt.Fprintln(os.Stderr, "error:", err)
	case profile == nil:
		fmt.Println("No tutor saved yet.")
	default:
		fmt.Print(profile)
	}
}

// startOver keeps a backup of the current design, then begins a new one with a fresh conversation.
func startOver(ctx context.Context, designer *agent.Agent, in *bufio.Scanner) error {
	if _, err := os.Stat(profilePath); err == nil {
		fmt.Printf("Start a new tutor? The current one is kept as %s. [y/N] ", backupPath)
		if !in.Scan() {
			return nil
		}
		if answer := strings.ToLower(strings.TrimSpace(in.Text())); answer != "y" && answer != "yes" {
			return nil
		}
		if err := os.Rename(profilePath, backupPath); err != nil {
			return err
		}
	}
	designer.Reset()
	return startDesign(ctx, designer)
}

// envOr returns the environment variable key, or def when it is unset or empty.
func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
