package tools

import (
	"bufio"
	"fmt"
	"strings"
)

const (
	diffContext   = 3         // unchanged lines shown around each change
	maxDiffCells  = 4_000_000 // limit on the comparison table, so a huge change can't hang the program
	maxShownLines = 200       // diff lines printed before the rest is summarized
)

// diffOp is one line of a diff: ' ' unchanged, '-' removed, '+' added.
type diffOp struct {
	kind byte
	text string
	line int // line number in the old file
}

// lineDiff returns a diff of before and after with a few lines of context around each change,
// or "" when they are the same. Each hunk starts with "@@ line N @@" (N is the line in the old file).
func lineDiff(before, after string) string {
	a, b := strings.Split(before, "\n"), strings.Split(after, "\n")
	if before == "" {
		a = nil // a new file: every line is added
	}

	// Lines that match at the start and end don't need the comparison table.
	pre := 0
	for pre < len(a) && pre < len(b) && a[pre] == b[pre] {
		pre++
	}
	suf := 0
	for suf < len(a)-pre && suf < len(b)-pre && a[len(a)-1-suf] == b[len(b)-1-suf] {
		suf++
	}
	midA, midB := a[pre:len(a)-suf], b[pre:len(b)-suf]
	if len(midA) == 0 && len(midB) == 0 {
		return ""
	}
	if len(midA)*len(midB) > maxDiffCells {
		return fmt.Sprintf("@@ line %d @@\n(change too large to show: %d lines replaced by %d)\n", pre+1, len(midA), len(midB))
	}

	var ops []diffOp
	for i := 0; i < pre; i++ {
		ops = append(ops, diffOp{' ', a[i], i + 1})
	}
	ops = append(ops, compareLines(midA, midB, pre+1)...)
	for i := len(a) - suf; i < len(a); i++ {
		ops = append(ops, diffOp{' ', a[i], i + 1})
	}
	return renderHunks(ops)
}

// compareLines finds the longest run of lines a and b share, and marks everything else removed or added.
func compareLines(a, b []string, firstLine int) []diffOp {
	// lcs[i*(m+1)+j] is how many lines a[i:] and b[j:] have in common.
	n, m := len(a), len(b)
	lcs := make([]int32, (n+1)*(m+1))
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if a[i] == b[j] {
				lcs[i*(m+1)+j] = lcs[(i+1)*(m+1)+j+1] + 1
			} else {
				lcs[i*(m+1)+j] = max(lcs[(i+1)*(m+1)+j], lcs[i*(m+1)+j+1])
			}
		}
	}

	var ops []diffOp
	i, j := 0, 0
	for i < n || j < m {
		switch {
		case i < n && j < m && a[i] == b[j]:
			ops = append(ops, diffOp{' ', a[i], firstLine + i})
			i, j = i+1, j+1
		case j == m || (i < n && lcs[(i+1)*(m+1)+j] >= lcs[i*(m+1)+j+1]):
			ops = append(ops, diffOp{'-', a[i], firstLine + i})
			i++
		default:
			ops = append(ops, diffOp{'+', b[j], firstLine + i})
			j++
		}
	}
	return ops
}

// renderHunks prints only the changed lines and the context around them.
func renderHunks(ops []diffOp) string {
	var b strings.Builder
	for i := 0; i < len(ops); {
		if ops[i].kind == ' ' {
			i++
			continue
		}
		// Grow the hunk while the next change is close enough to share context with this one.
		last := i
		for j := i; j < len(ops) && j-last <= 2*diffContext; j++ {
			if ops[j].kind != ' ' {
				last = j
			}
		}
		start, stop := max(0, i-diffContext), min(len(ops), last+diffContext+1)
		fmt.Fprintf(&b, "@@ line %d @@\n", ops[start].line)
		for _, o := range ops[start:stop] {
			b.WriteByte(o.kind)
			b.WriteString(strings.TrimSuffix(o.text, "\r"))
			b.WriteByte('\n')
		}
		i = stop
	}
	return b.String()
}

// approve shows the user what a write will change and asks them to allow it.
func approve(in *bufio.Scanner, action, path, before, after string) bool {
	fmt.Printf("\nClaude wants to %s %s:\n", action, path)
	printDiff(lineDiff(before, after))
	fmt.Print("Allow? [y/N] ")
	if !in.Scan() {
		return false
	}
	answer := strings.ToLower(strings.TrimSpace(in.Text()))
	return answer == "y" || answer == "yes"
}

// printDiff shows a diff in the terminal, removed lines in red and added lines in green.
func printDiff(diff string) {
	if diff == "" {
		fmt.Println("  (no changes)")
		return
	}
	lines := strings.Split(strings.TrimSuffix(diff, "\n"), "\n")
	for i, line := range lines {
		if i == maxShownLines {
			fmt.Printf("  ... %d more diff lines not shown\n", len(lines)-i)
			return
		}
		switch line[0] {
		case '-':
			fmt.Printf("\033[31m%s\033[0m\n", line)
		case '+':
			fmt.Printf("\033[32m%s\033[0m\n", line)
		case '@':
			fmt.Printf("\033[36m%s\033[0m\n", line)
		default:
			fmt.Println(line)
		}
	}
}
