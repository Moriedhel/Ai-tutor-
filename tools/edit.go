package tools

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"strings"

	"github.com/anthropics/anthropic-sdk-go/v2"
)

// EditFile lets Claude change one exact piece of text in a file, instead of rewriting the whole file.
// The user sees the diff and approves it first.
type EditFile struct {
	Dir FilesDir
	In  *bufio.Scanner // where the user's y/N answer is read from
}

func (EditFile) Definition() anthropic.ToolParam {
	return anthropic.ToolParam{
		Name: "edit_file",
		Description: anthropic.String("Change part of an existing file by replacing old_text with new_text. " +
			"old_text must match the file exactly (including indentation) and appear exactly once; " +
			"include a few surrounding lines if needed to make it unique. Read the file first. " +
			"Use write_file to create new files. The user sees the diff and must approve it, and may say no."),
		Strict: anthropic.Bool(true),
		InputSchema: anthropic.ToolInputSchemaParam{
			Properties: map[string]any{
				"path":     map[string]any{"type": "string", "description": "File to edit, relative to the files folder."},
				"old_text": map[string]any{"type": "string", "description": "The exact text to replace."},
				"new_text": map[string]any{"type": "string", "description": "The text to put in its place."},
			},
			Required:    []string{"path", "old_text", "new_text"},
			ExtraFields: map[string]any{"additionalProperties": false},
		},
	}
}

func (e EditFile) Run(_ context.Context, raw json.RawMessage) (string, error) {
	var in struct {
		Path    string `json:"path"`
		OldText string `json:"old_text"`
		NewText string `json:"new_text"`
	}
	if err := json.Unmarshal(raw, &in); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}
	if in.OldText == "" {
		return "", errors.New("old_text must not be empty; use write_file to create or replace a whole file")
	}
	p, err := e.Dir.clean(in.Path)
	if err != nil {
		return "", err
	}
	data, err := fs.ReadFile(e.Dir.root.FS(), p)
	if errors.Is(err, fs.ErrNotExist) {
		return "", fmt.Errorf("%s does not exist; use write_file to create it", p)
	}
	if err != nil {
		return "", err
	}
	before := string(data)

	// Files saved on Windows often use \r\n line endings, while Claude writes \n.
	oldText, newText := in.OldText, in.NewText
	if !strings.Contains(before, oldText) && strings.Contains(before, "\r\n") {
		oldText, newText = toCRLF(oldText), toCRLF(newText)
	}
	switch n := strings.Count(before, oldText); {
	case n == 0:
		return "", fmt.Errorf("old_text was not found in %s; read the file again and copy the text exactly", p)
	case n > 1:
		return "", fmt.Errorf("old_text appears %d times in %s; include more surrounding lines so it matches once", n, p)
	}
	after := strings.Replace(before, oldText, newText, 1)

	if !approve(e.In, "edit", p, before, after) {
		return "The user did not allow this edit. Nothing was changed.", nil
	}
	if err := e.Dir.root.WriteFile(p, []byte(after), 0o644); err != nil {
		return "", err
	}
	fmt.Printf("(edited %s)\n", p)
	return "Edited " + p + ".", nil
}

// toCRLF turns every line ending in s into \r\n.
func toCRLF(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "\r\n", "\n"), "\n", "\r\n")
}
