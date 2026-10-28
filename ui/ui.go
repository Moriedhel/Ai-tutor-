// Package ui holds the terminal colors, so everything Claude says looks the same wherever it's printed.
package ui

// ANSI color codes. Text printed after one of these is colored until Reset.
const (
	Claude = "\033[36m" // cyan: Claude's replies and the questions it asks you
	Reset  = "\033[0m"
)
