# tutor-agent

A terminal chat with Claude, written in Go, that helps you design a tutor. Built step by step with a hand-written agent harness.

## Run it

Copy `.env.example` to `.env` and put your API key in it (from console.anthropic.com), then:

```bash
go run .
```

Settings in `.env` (environment variables work too):

- `ANTHROPIC_API_KEY`: required. `.env` is gitignored, so the key never gets committed.
- `MODEL`: which Claude model to use. Defaults to `claude-opus-5-5`.
- `FILES_DIR`: the folder the agent can work in. Defaults to the project folder. Use forward slashes on Windows (`C:/Users/you/notes`). The agent can't leave that folder or touch `.env`.

## Using it

When it starts, the agent walks you through designing a tutor one step at a time: subject, audience, teaching style, lesson ideas, then a name. Claude's messages are shown in cyan, so you can tell them apart from yours.

When Claude offers choices you get a numbered menu. At the menu you can:

- type a number to pick an option
- type your own answer
- press Enter on an empty line to skip the question and ask something else at the `you>` prompt
- type a command (below); it runs as soon as Claude's reply ends

Your design is saved to `tutor_profile.json` after every decision. Quit whenever you like: next time you run it, the agent shows what's saved and picks up at the first undecided step.

Commands, at the `you>` prompt or at a menu:

- `/profile` shows the design so far
- `/new` starts a new tutor (the old one is kept as `tutor_profile.bak.json`)
- `/quit` exits

You can also ask about your files or code at any time, for example "read my notes in course.md" or "add a comment to envOr in main.go, then build and test".

## What the agent can do

| Tool | What it does | Asks you first? |
|---|---|---|
| `propose_options` | shows a numbered menu and returns your answer | it is the question |
| `save_tutor_profile` | saves the design to `tutor_profile.json` | no |
| `list_files`, `read_file` | look around and read text files in `FILES_DIR` | no |
| `write_file` | creates, replaces or appends to a file | yes, with a diff |
| `edit_file` | replaces one exact piece of text in a file | yes, with a diff |
| `run_check` | runs `go build`, `go vet` or `go test` on `./...` in `FILES_DIR` | no |
| `go_doc` | looks up Go documentation with `go doc` | no |

Before every write or edit you see a diff (removed lines in red, added in green) and answer y/N. `run_check` and `go_doc` run fixed commands only: Claude chooses which check or which symbol, never the program or its options, and the API key is removed from their environment. The agent has no general shell and can't run git.

## How the harness works

```
main.go            startup, the chat prompt, the / commands, resuming a saved design
tools.go           allTools: which tools this program gives the agent, and deps they need
system_prompt.md   the instructions Claude gets in every chat (built into the program; edit and rerun)
agent/             the harness: the Tool interface and the loop
tools/             everything the agent can do, one file per group:
                     options.go  profile.go  files.go  edit.go  diff.go  check.go  godoc.go
tutor/             the tutor profile: loading, saving, and what's still undecided
ui/                terminal colors
```

- `agent`: send the conversation to Claude, stream the reply, and if Claude asked for tools, run them, send the results back, and repeat until Claude is done. It knows nothing about tutors.
- `tutor`: the data being designed. It doesn't depend on Claude.
- `tools`: each tool has a definition (name, description, input schema) and a `Run` function. Tools don't import `agent`; any type with those two methods is an `agent.Tool`. What `Run` returns goes to Claude, not to you: if you should see something, Claude has to say it in its reply.
- `ui`: one place for colors. Change `ui.Claude` to give Claude's messages a different color.

### Adding a tool

1. Write a new type with `Definition()` and `Run()` in `tools/`.
2. Add it to `allTools` in `tools.go`. If it needs something built at startup (the input scanner, the files folder), add it to `deps` and fill it in `main.go`.
3. If Claude should use it in a particular way, say so in its description; mention it in `system_prompt.md` only for how it fits the conversation.

To wrap a command-line program, copy `tools/godoc.go`: the program and its options are fixed in the code, Claude's input is passed as one argument (never through a shell), and output has a timeout.

Run the tests with `go test ./...`.

## Roadmap

1. Minimal chat CLI with streaming and conversation history (done)
2. Own harness loop with tools (`propose_options`, `save_tutor_profile`) (done)
3. Tutor-design mode that saves a profile you can resume (done)
4. Chat with the tutor you designed
