// Package tools holds everything the agent can do. Each type here satisfies agent.Tool.
package tools

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/anthropics/anthropic-sdk-go/v2"

	"tutor-agent/ui"
)

// ProposeOptions shows the user a numbered list of ideas and returns what they picked.
type ProposeOptions struct {
	In *bufio.Scanner // where the user's answer is read from
	// OnCommand receives a /command typed at the menu. The program runs it once Claude's reply has ended.
	OnCommand func(command string)
}

func (ProposeOptions) Definition() anthropic.ToolParam {
	return anthropic.ToolParam{
		Name: "propose_options",
		Description: anthropic.String("Show the user a numbered list of options and wait for their choice. " +
			"Use this whenever the user needs to decide something about their tutor. " +
			"Returns the option they picked, their own free-text answer, or that they skipped the question."),
		Strict: anthropic.Bool(true),
		InputSchema: anthropic.ToolInputSchemaParam{
			Properties: map[string]any{
				"question": map[string]any{"type": "string", "description": "The question to ask the user."},
				"options": map[string]any{
					"type":        "array",
					"items":       map[string]any{"type": "string"},
					"description": "Two to six short options.",
				},
			},
			Required:    []string{"question", "options"},
			ExtraFields: map[string]any{"additionalProperties": false},
		},
	}
}

func (p ProposeOptions) Run(_ context.Context, raw json.RawMessage) (string, error) {
	var in struct {
		Question string   `json:"question"`
		Options  []string `json:"options"`
	}
	if err := json.Unmarshal(raw, &in); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}
	if len(in.Options) == 0 {
		return "", errors.New("options must not be empty")
	}

	// The question and options are Claude's words, so they get Claude's color; the prompt line is the user's.
	fmt.Printf("\n%s%s\n", ui.Claude, in.Question)
	for i, opt := range in.Options {
		fmt.Printf("  %d. %s\n", i+1, opt)
	}
	fmt.Print(ui.Reset)
	fmt.Print("Pick a number, type your own answer, or press Enter to skip and ask something else: ")
	if !p.In.Scan() {
		return "", errors.New("no answer from the user")
	}
	answer := strings.TrimSpace(p.In.Text())
	if answer == "" {
		// The user's way out of the menu: Claude stops, and the user gets the normal you> prompt.
		return "The user skipped this question because they want to ask or say something else. " +
			"Do not ask it again now. End your reply here and wait for their next message.", nil
	}
	if strings.HasPrefix(answer, "/") && p.OnCommand != nil {
		p.OnCommand(answer)
		return "The user typed the program command " + answer + ", which the program will run after your reply. " +
			"Do not ask this question again now, and don't comment on the command. End your reply here.", nil
	}
	if n, err := strconv.Atoi(answer); err == nil && n >= 1 && n <= len(in.Options) {
		return "The user picked: " + in.Options[n-1], nil
	}
	return "The user did not pick an option; they typed: " + strconv.Quote(answer) + "\n" +
		"If this is their choice, use it. If it is a question or another request, do that instead, " +
		"show them the result in your reply, and end your reply without asking a design question.", nil
}
