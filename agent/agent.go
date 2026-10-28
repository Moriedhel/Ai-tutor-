// Package agent is the harness: it sends the conversation to Claude, runs the tools
// Claude asks for, and repeats until Claude gives its final answer.
package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/anthropics/anthropic-sdk-go/v2"

	"tutor-agent/ui"
)

// Tool is one capability the agent can use. The harness shows Claude each
// tool's definition, and runs the tool when Claude asks for it.
type Tool interface {
	Definition() anthropic.ToolParam
	// Run executes the tool with Claude's JSON input and returns the text sent back as the tool result.
	Run(ctx context.Context, input json.RawMessage) (string, error)
}

// Config is what the agent sends to Claude with every request.
type Config struct {
	Model     anthropic.Model
	MaxTokens int64
	System    string // the system prompt
}

// Agent is the harness: it owns the conversation, the tools, and the loop
// that keeps calling Claude until it stops asking for tools.
type Agent struct {
	client  anthropic.Client
	config  Config
	tools   map[string]Tool
	defs    []anthropic.ToolUnionParam // kept in a fixed order so requests stay identical turn to turn
	history []anthropic.MessageParam
}

func New(client anthropic.Client, config Config, tools ...Tool) *Agent {
	a := &Agent{client: client, config: config, tools: map[string]Tool{}}
	for _, t := range tools {
		def := t.Definition()
		a.tools[def.Name] = t
		a.defs = append(a.defs, anthropic.ToolUnionParam{OfTool: &def})
	}
	return a
}

// Reset forgets the conversation so far. Tools and the system prompt stay the same.
func (a *Agent) Reset() {
	a.history = nil
}

// Turn handles one user message. Claude may call tools several times before giving its final answer.
func (a *Agent) Turn(ctx context.Context, userText string) error {
	start := len(a.history)
	a.history = append(a.history, anthropic.NewUserMessage(anthropic.NewTextBlock(userText)))

	for {
		msg, err := a.streamReply(ctx)
		if err != nil {
			a.history = a.history[:start] // drop the failed turn so you can retry
			return err
		}
		// Append the full content (not just the text) so tool calls and thinking blocks stay intact.
		a.history = append(a.history, msg.ToParam())

		switch msg.StopReason {
		case anthropic.StopReasonToolUse:
			a.history = append(a.history, anthropic.NewUserMessage(a.runTools(ctx, msg)...))
		case anthropic.StopReasonRefusal:
			fmt.Println("(Claude declined to answer that one.)")
			return nil
		case anthropic.StopReasonMaxTokens:
			fmt.Println("(Reply hit the length limit and was cut off.)")
			return nil
		default:
			return nil
		}
	}
}

// runTools executes every tool call in Claude's message and returns all results for one user turn.
func (a *Agent) runTools(ctx context.Context, msg *anthropic.Message) []anthropic.ContentBlockParamUnion {
	var results []anthropic.ContentBlockParamUnion
	for _, block := range msg.Content {
		use, ok := block.AsAny().(anthropic.ToolUseBlock)
		if !ok {
			continue
		}
		tool, ok := a.tools[use.Name]
		if !ok {
			results = append(results, anthropic.NewToolResultBlock(use.ID, "unknown tool: "+use.Name, true))
			continue
		}
		out, err := tool.Run(ctx, json.RawMessage(use.JSON.Input.Raw()))
		if err != nil {
			results = append(results, anthropic.NewToolResultBlock(use.ID, err.Error(), true))
			continue
		}
		results = append(results, anthropic.NewToolResultBlock(use.ID, out, false))
	}
	return results
}

// streamReply sends the conversation so far and prints Claude's text as it streams in.
func (a *Agent) streamReply(ctx context.Context) (*anthropic.Message, error) {
	stream := a.client.Messages.NewStreaming(ctx, anthropic.MessageNewParams{
		Model:     a.config.Model,
		MaxTokens: a.config.MaxTokens,
		System:    []anthropic.TextBlockParam{{Text: a.config.System}},
		Messages:  a.history,
		Tools:     a.defs,
	})

	message := anthropic.Message{}
	var out replyPrinter
	defer out.done()
	for stream.Next() {
		event := stream.Current()
		if err := message.Accumulate(event); err != nil {
			return nil, err
		}
		if delta, ok := event.AsAny().(anthropic.ContentBlockDeltaEvent); ok {
			if text, ok := delta.Delta.AsAny().(anthropic.TextDelta); ok {
				out.print(text.Text)
			}
		}
	}
	if err := stream.Err(); err != nil {
		var apiErr *anthropic.Error
		if errors.As(err, &apiErr) {
			return nil, fmt.Errorf("API error %d: %w", apiErr.StatusCode, err)
		}
		return nil, err
	}
	return &message, nil
}

// replyPrinter prints Claude's text in Claude's color. The "claude>" label appears with the first
// text, so a reply that only calls tools prints nothing.
type replyPrinter struct{ started bool }

func (p *replyPrinter) print(text string) {
	if !p.started {
		fmt.Print("\n" + ui.Claude + "claude> ") // stays colored until done
		p.started = true
	}
	fmt.Print(text)
}

// done switches back to the normal color and ends the line.
func (p *replyPrinter) done() {
	if p.started {
		fmt.Println(ui.Reset)
	}
}
