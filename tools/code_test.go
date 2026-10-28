package tools

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// testDir makes a files folder with the given files, closed again when the test ends.
func testDir(t *testing.T, files map[string]string) (FilesDir, string) {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	d, err := OpenFilesDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.root.Close() }) // Windows can't delete a folder that is still open
	return d, dir
}

func answers(s string) *bufio.Scanner { return bufio.NewScanner(strings.NewReader(s)) }

func TestLineDiff(t *testing.T) {
	before := "a\nb\nc\nd\ne\nf\ng\nh\n"
	after := "a\nb\nc\nD\ne\nf\ng\nh\n"
	want := "@@ line 1 @@\n a\n b\n c\n-d\n+D\n e\n f\n g\n"
	if got := lineDiff(before, after); got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
	if got := lineDiff(before, before); got != "" {
		t.Fatalf("same text should give no diff, got %q", got)
	}
	if got := lineDiff("", "x\n"); !strings.Contains(got, "+x") {
		t.Fatalf("new file should show every line added, got %q", got)
	}
}

func TestEditFile(t *testing.T) {
	d, dir := testDir(t, map[string]string{
		"main.go":   "package main\n\nfunc main() {\n\tprintln(\"hi\")\n}\n",
		"crlf.go":   "package main\r\n\r\nvar x = 1\r\n",
		"twice.txt": "same\nsame\n",
	})
	edit := func(answer, path, oldText, newText string) (string, error) {
		raw, _ := json.Marshal(map[string]string{"path": path, "old_text": oldText, "new_text": newText})
		return EditFile{Dir: d, In: answers(answer)}.Run(context.Background(), raw)
	}

	if _, err := edit("y\n", "main.go", `println("hi")`, `println("hello")`); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "main.go")); !strings.Contains(string(b), `println("hello")`) {
		t.Fatalf("edit not applied:\n%s", b)
	}

	if _, err := edit("y\n", "crlf.go", "\nvar x = 1\n", "\nvar x = 2\n"); err != nil {
		t.Fatalf("\\n text should match a \\r\\n file: %v", err)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "crlf.go")); string(b) != "package main\r\n\r\nvar x = 2\r\n" {
		t.Fatalf("line endings not kept: %q", b)
	}

	if _, err := edit("y\n", "main.go", "not there", "x"); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("missing text should fail, got %v", err)
	}
	if _, err := edit("y\n", "twice.txt", "same", "x"); err == nil || !strings.Contains(err.Error(), "2 times") {
		t.Fatalf("ambiguous text should fail, got %v", err)
	}

	out, err := edit("n\n", "main.go", "hello", "bye")
	if err != nil || !strings.Contains(out, "did not allow") {
		t.Fatalf("saying no should leave the file alone, got %q, %v", out, err)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "main.go")); strings.Contains(string(b), "bye") {
		t.Fatal("file changed although the user said no")
	}
}

func TestRunCheck(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the go command")
	}
	d, _ := testDir(t, map[string]string{
		"go.mod":      "module example\n\ngo 1.25\n",
		"main.go":     "package main\n\nfunc main() {}\n",
		"bad_test.go": "package main\n\nimport \"testing\"\n\nfunc TestFails(t *testing.T) { t.Fatal(\"boom\") }\n",
	})
	run := func(check string) string {
		out, err := RunCheck{Dir: d}.Run(context.Background(), json.RawMessage(`{"check":"`+check+`"}`))
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	if out := run("build"); !strings.HasPrefix(out, "OK") {
		t.Fatalf("build should pass, got %q", out)
	}
	if out := run("test"); !strings.HasPrefix(out, "FAILED") || !strings.Contains(out, "boom") {
		t.Fatalf("test should fail with the test's message, got %q", out)
	}
}
