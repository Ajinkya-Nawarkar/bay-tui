# bay

A terminal session manager for developers who run multiple repos, branches, and AI agents side by side.

bay groups your shells into **sessions** organized by repo. Switch between projects instantly. Restore your exact pane layout on restart. Keep Claude Code agents running alongside your regular terminals.

## Why bay?

- **One keystroke to switch context** — jump between repos and sessions without losing your place
- **Pane layout persists across restarts** — quit and come back to exactly where you left off
- **AI agents as first-class panes** — split a Claude Code agent next to your shell with `` `+a ``
- **Session notes** — annotate what you're working on so you remember days later
- **Built-in memory system** — sessions track episodic history and working state across agent conversations

## Context & Memory

The hardest part of working across multiple sessions isn't the code — it's remembering where you left off.

bay solves this at three levels:

**Session notes** — When you're deep in a debugging session, hit `N` and jot down what you're doing. Come back two days later and instantly know: "OAuth2 flow — stuck on token refresh, check middleware." No more `git log` archaeology to figure out what past-you was thinking.

**Episodic history** — bay automatically records session events: when you activated a session, switched away, what your panes were doing. Search across all of it with `bay search "auth bug"` to find that terminal output from last Tuesday.

**Working state** — Each session tracks its current task, git branch, and last summary. When you spin up a Claude Code agent, bay injects this context automatically — the agent already knows what you're working on, what other sessions are doing in the same repo, and recent activity. No copy-pasting, no "here's what I was doing" preamble.

**Context injection rules** — Define files that should be injected into agent conversations per-repo with `bay rules add`. Design docs, API specs, coding standards — your agents start every conversation with the right context.

```bash
bay mem show              # See current session's memory state
bay mem task "fix auth"   # Set what you're working on
bay mem note "try v2 API" # Quick note for future reference
bay search "migration"    # Full-text search across all session history
bay rules add DESIGN.md   # Inject into agent context for this repo
```

The result: you switch between five repos, quit for the day, come back tomorrow, and every session remembers exactly where you were. Your agents pick up mid-conversation like nothing happened.

## Quick Start

```bash
# Install
go install github.com/Ajinkya-Nawarkar/bay-tui@latest

# Or build from source
git clone https://github.com/Ajinkya-Nawarkar/bay-tui.git
cd bay-tui && go build -o bay . && sudo mv bay /usr/local/bin/

# Launch
bay
```

On first run, bay walks you through setup — point it at your workspace directory and you're ready.

## How It Works

bay runs inside a single tmux session. A persistent topbar shows your repos and sessions:

```
╭─────────────────────────────────────────────────────╮
│ bay   my-api │ frontend │ infra                     │
│       [1:main*] [2:feature-auth] [3:hotfix]         │
│       Implementing OAuth2 flow for /login endpoint  │
╰─────────────────────────────────────────────────────╯
```

Each session owns a tmux window with your shell panes and agents. Switch sessions and bay moves the topbar, restores focus, and tracks everything.

## Key Bindings

All shortcuts use `` ` `` (backtick) as the prefix key.

| Keys | Action |
|------|--------|
| `` `+space `` | Toggle focus mode (interact with topbar) |
| `` `+tab `` | Cycle to next session |
| `` `+1-9 `` | Jump to session by number |
| `` `+r `` | Cycle repos |
| `` `+d `` | Vertical split |
| `` `+D `` | Horizontal split |
| `` `+a `` | Open Claude Code agent pane |
| `` `+w `` | Close pane |
| `` `+{/} `` | Swap pane position |
| `` `+arrows `` | Navigate between panes |

In **focus mode** (`` `+space ``):

| Keys | Action |
|------|--------|
| `n` | New session |
| `d` | Delete session |
| `R` | Rename session |
| `N` | Edit session note |
| `q` | Quit bay |

## Commands

```
bay              Launch bay
bay -f           Fresh start (kill existing, relaunch)
bay setup        Run setup wizard
bay ls           List sessions
bay kill <name>  Kill a session
bay keybinds     Keybind reference
bay mem show     Show session memory
bay search "q"   Search across session history
```

## Requirements

- Go 1.25+
- tmux 3.2+
- macOS or Linux

## License

MIT
