# bay

A terminal session manager for developers who run multiple repos, branches, and AI agents side by side.

You're juggling 3 repos. You've got a half-finished auth flow in one, a migration in another, and a hotfix you started yesterday. Here's your morning:

**Without bay:**
```
$ tmux ls                          # which session was which?
$ cd ~/work/my-api && git log      # what was I doing here?
$ claude                           # "so here's the context..."
                                   # *pastes 40 lines of background*
```

**With bay:**
```
$ bay
```

That's it. You're back. Every session, every pane, every agent — exactly where you left them. The topbar tells you what's going on at a glance:

```
╭──────────────────────────────────────────────────────────────╮
│ bay   my-api │ frontend │ infra                              │
│       [1:main*] [2:feature-auth] [3:hotfix]                  │
│       OAuth2 flow — stuck on token refresh, check middleware │
╰──────────────────────────────────────────────────────────────╯
┌ shell ─────────────────────┐┌ claude ─────────────────────────┐
│ ~/my-api (feature-auth) $  ││ Claude Code ready.              │
│                            ││ Session context loaded:          │
│                            ││  - task: fix auth flow           │
│                            ││  - sibling: hotfix on main       │
│                            ││  - last: refactored middleware   │
└────────────────────────────┘└─────────────────────────────────┘
```

The agent already knows what you're working on. It read the session memory. You didn't paste anything.

**Three things that matter:**

1. `` `+a `` splits a Claude Code agent that inherits your session's full context — task, branch, sibling activity, history
2. `` `+1 `` through `` `+9 `` jumps between sessions. `` `+r `` switches repos. Zero friction.
3. Quit with `q`, come back tomorrow — pane layout, notes, memory, all restored

bay is the workspace layer between you and tmux that makes multi-repo, multi-agent development feel like one continuous session instead of 15 disconnected terminals.

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

## Tasks

Set up what needs doing, assign agents to specific tasks, and track progress — all from the terminal.

```bash
bay task "Fix auth flow"              # Create a task
bay task add "Write tests" -p 1       # Add subtask under task #1
bay task ls                           # List all tasks with status
bay task doing 1                      # Mark in-progress
bay task done 1                       # Mark done
bay task assign 2                     # Assign current pane to task #2
bay task clear                        # Clear all tasks
```

Tasks are injected into agent context on startup. When a pane is assigned to a task, the agent sees which task it's responsible for.

## Context & Memory

**Session notes** — Hit `N` and jot down what you're doing. Come back two days later and instantly know: "OAuth2 flow — stuck on token refresh, check middleware."

**Episodic history** — bay automatically records session events. Search across all of it with `bay ctx search "auth bug"` to find that terminal output from last Tuesday.

**Working state** — Each session tracks its git branch and last summary. Spin up a Claude Code agent and bay injects this context automatically — no copy-pasting, no preamble.

**Context injection rules** — Define files that get injected into agent conversations per-repo with `bay ctx add`. Design docs, API specs, coding standards — your agents start every conversation with the right context.

```bash
bay ctx show              # See current session's memory state
bay ctx search "migration"  # Full-text search across all session history
bay ctx add design DESIGN.md  # Inject into agent context for this repo
```

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
bay                  Launch bay
bay -f               Fresh start (kill existing, relaunch)

bay session ls       List all sessions
bay session kill <n> Kill a session

bay task "desc"      Create a task
bay task ls          List tasks
bay task done <id>   Mark done
bay task assign <id> Assign pane to task

bay ctx show         Show session memory state
bay ctx search "q"   Search across session history
bay ctx files        List context files
bay ctx add <n> <p>  Register a context file

bay setup            Run setup wizard
bay keybinds         Keybind reference
bay build            Rebuild from source
bay upgrade          Download latest release
bay uninstall        Remove all bay data
```

## Requirements

- Go 1.25+
- tmux 3.2+
- macOS or Linux

## License

MIT
