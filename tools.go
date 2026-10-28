package main

import (
	"bufio"

	"tutor-agent/agent"
	"tutor-agent/tools"
)

// deps holds what tools need from startup: things built in main that tools can't create themselves.
type deps struct {
	in        *bufio.Scanner
	files     tools.FilesDir
	onCommand func(command string) // queues a /command typed inside a tool, for the main loop to run
}

// allTools is the one place a new tool gets added. The tools themselves live in the tools package.
func allTools(d deps) []agent.Tool {
	return []agent.Tool{
		tools.ProposeOptions{In: d.in, OnCommand: d.onCommand},
		tools.SaveProfile{Path: profilePath},
		tools.ListFiles{Dir: d.files},
		tools.ReadFile{Dir: d.files},
		tools.WriteFile{Dir: d.files, In: d.in},
		tools.EditFile{Dir: d.files, In: d.in},
		tools.RunCheck{Dir: d.files},
		tools.GoDoc{Dir: d.files},
	}
}
