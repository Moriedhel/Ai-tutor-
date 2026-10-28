package tools

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/anthropics/anthropic-sdk-go/v2"
)

// maxFileBytes caps how much of a file read_file returns, so one big file can't flood the conversation.
const maxFileBytes = 100_000

// FilesDir gives the file tools a root folder they can't escape (no "..", no symlinks out).
type FilesDir struct {
	root   *os.Root
	osPath string // the folder on disk, for running commands in it
}

// OpenFilesDir opens the folder the file tools work in.
func OpenFilesDir(path string) (FilesDir, error) {
	root, err := os.OpenRoot(path)
	if err != nil {
		return FilesDir{}, err
	}
	return FilesDir{root: root, osPath: path}, nil
}

// clean turns Claude's path into one fs.FS accepts, and refuses secrets like .env.
func (d FilesDir) clean(p string) (string, error) {
	p = path.Clean(strings.TrimPrefix(filepath.ToSlash(p), "/"))
	if p == "" {
		p = "."
	}
	if !fs.ValidPath(p) {
		return "", fmt.Errorf("path %q is outside the files folder", p)
	}
	if path.Base(p) == ".env" {
		return "", errors.New(".env holds secrets and can't be read or written")
	}
	return p, nil
}

// ListFiles shows Claude what's in a folder.
type ListFiles struct{ Dir FilesDir }

func (ListFiles) Definition() anthropic.ToolParam {
	return anthropic.ToolParam{
		Name: "list_files",
		Description: anthropic.String("List the files and folders in the user's files folder. " +
			"Folders end with a slash. Use \".\" for the top level."),
		Strict: anthropic.Bool(true),
		InputSchema: anthropic.ToolInputSchemaParam{
			Properties: map[string]any{
				"path": map[string]any{"type": "string", "description": "Folder to list, relative to the files folder."},
			},
			Required:    []string{"path"},
			ExtraFields: map[string]any{"additionalProperties": false},
		},
	}
}

func (l ListFiles) Run(_ context.Context, raw json.RawMessage) (string, error) {
	var in struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(raw, &in); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}
	p, err := l.Dir.clean(in.Path)
	if err != nil {
		return "", err
	}
	entries, err := fs.ReadDir(l.Dir.root.FS(), p)
	if err != nil {
		return "", err
	}
	if len(entries) == 0 {
		return "(empty folder)", nil
	}
	var b strings.Builder
	for _, e := range entries {
		b.WriteString(e.Name())
		if e.IsDir() {
			b.WriteString("/")
		}
		b.WriteString("\n")
	}
	return b.String(), nil
}

// ReadFile gives Claude the text of one file.
type ReadFile struct{ Dir FilesDir }

func (ReadFile) Definition() anthropic.ToolParam {
	return anthropic.ToolParam{
		Name: "read_file",
		Description: anthropic.String("Read a text file from the user's files folder. " +
			"Use list_files first if you don't know the exact path. Very long files are cut off."),
		Strict: anthropic.Bool(true),
		InputSchema: anthropic.ToolInputSchemaParam{
			Properties: map[string]any{
				"path": map[string]any{"type": "string", "description": "File to read, relative to the files folder."},
			},
			Required:    []string{"path"},
			ExtraFields: map[string]any{"additionalProperties": false},
		},
	}
}

func (r ReadFile) Run(_ context.Context, raw json.RawMessage) (string, error) {
	var in struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(raw, &in); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}
	p, err := r.Dir.clean(in.Path)
	if err != nil {
		return "", err
	}
	data, err := fs.ReadFile(r.Dir.root.FS(), p)
	if err != nil {
		return "", err
	}
	if !utf8.Valid(data) {
		return "", fmt.Errorf("%s is not a text file", p)
	}
	fmt.Printf("\n(read %s)\n", p)
	if len(data) > maxFileBytes {
		return string(data[:maxFileBytes]) + fmt.Sprintf("\n\n[cut off: showing %d of %d bytes]", maxFileBytes, len(data)), nil
	}
	return string(data), nil
}

// WriteFile lets Claude create a file, replace one, or add to the end of one.
// The user approves every write in the terminal first.
type WriteFile struct {
	Dir FilesDir
	In  *bufio.Scanner // where the user's y/N answer is read from
}

func (WriteFile) Definition() anthropic.ToolParam {
	return anthropic.ToolParam{
		Name: "write_file",
		Description: anthropic.String("Write a text file in the user's files folder. " +
			"mode \"overwrite\" creates the file or replaces everything in it; " +
			"mode \"append\" adds to the end (creating the file if needed). " +
			"Missing folders are created. The user must approve each write, and may say no."),
		Strict: anthropic.Bool(true),
		InputSchema: anthropic.ToolInputSchemaParam{
			Properties: map[string]any{
				"path":    map[string]any{"type": "string", "description": "File to write, relative to the files folder."},
				"content": map[string]any{"type": "string", "description": "The text to write."},
				"mode":    map[string]any{"type": "string", "enum": []string{"overwrite", "append"}},
			},
			Required:    []string{"path", "content", "mode"},
			ExtraFields: map[string]any{"additionalProperties": false},
		},
	}
}

func (w WriteFile) Run(_ context.Context, raw json.RawMessage) (string, error) {
	var in struct {
		Path    string `json:"path"`
		Content string `json:"content"`
		Mode    string `json:"mode"`
	}
	if err := json.Unmarshal(raw, &in); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}
	p, err := w.Dir.clean(in.Path)
	if err != nil {
		return "", err
	}
	if p == "." {
		return "", errors.New("path must name a file")
	}

	// Work out the whole new file first, so the user can see exactly what changes.
	action, before := "create", ""
	data, err := fs.ReadFile(w.Dir.root.FS(), p)
	switch {
	case err == nil:
		if !utf8.Valid(data) {
			return "", fmt.Errorf("%s is not a text file", p)
		}
		action, before = "REPLACE", string(data)
	case !errors.Is(err, fs.ErrNotExist):
		return "", err
	}
	var after string
	switch in.Mode {
	case "overwrite":
		after = in.Content
	case "append":
		after = before + in.Content
		if action != "create" {
			action = "append to"
		}
	default:
		return "", fmt.Errorf("unknown mode %q", in.Mode)
	}

	if !approve(w.In, action, p, before, after) {
		return "The user did not allow this write. Nothing was changed.", nil
	}
	if dir := path.Dir(p); dir != "." {
		if err := w.Dir.root.MkdirAll(dir, 0o755); err != nil {
			return "", err
		}
	}
	if err := w.Dir.root.WriteFile(p, []byte(after), 0o644); err != nil {
		return "", err
	}
	fmt.Printf("(wrote %s)\n", p)
	return fmt.Sprintf("Wrote %d bytes to %s (%s).", len(in.Content), p, in.Mode), nil
}
