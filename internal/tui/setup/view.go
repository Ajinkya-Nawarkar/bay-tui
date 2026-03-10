package setup

import (
	"fmt"

	"bay/internal/tui/styles"
)

// View renders the setup wizard.
func (m Model) View() string {
	switch m.step {
	case stepWelcome:
		return styles.AppContainer.Render(
			styles.Title.Render(" bay") + "\n" +
				styles.HelpBar.Render("by Ajinkya Nawarkar <3") + "\n\n" +
				"Welcome to bay! Let's configure your workspace.\n\n" +
				"bay organizes your dev sessions by repo, with built-in\n" +
				"git worktree management and tmux integration.\n\n" +
				styles.HelpBar.Render("Press Enter to continue, q to quit"),
		)

	case stepScanDir:
		return styles.AppContainer.Render(
			styles.Title.Render(" bay setup") + "\n\n" +
				styles.InputLabel.Render("Where are your repos?") + "\n" +
				"Enter a directory to scan for git repositories:\n\n" +
				m.scanDirInput.View() + "\n\n" +
				styles.HelpBar.Render("Press Enter to continue"),
		)

	case stepWorktreeLocation:
		managed := "  1. managed  — worktrees in ~/.bay/worktrees/"
		adjacent := "  2. adjacent — worktrees next to the repo"
		if m.worktreeChoice == 0 {
			managed = styles.SelectedLine.Render(managed)
		} else {
			adjacent = styles.SelectedLine.Render(adjacent)
		}
		return styles.AppContainer.Render(
			styles.Title.Render(" bay setup") + "\n\n" +
				styles.InputLabel.Render("Where should worktrees be created?") + "\n\n" +
				managed + "\n" +
				adjacent + "\n\n" +
				styles.HelpBar.Render("Press 1 or 2 to select, Enter to confirm"),
		)

	case stepAgentCmd:
		return styles.AppContainer.Render(
			styles.Title.Render(" bay setup") + "\n\n" +
				styles.InputLabel.Render("What command should `+a run?") + "\n" +
				"Enter the CLI agent to launch in splits:\n\n" +
				m.agentInput.View() + "\n\n" +
				styles.HelpBar.Render("Press Enter to continue"),
		)

	case stepDone:
		if m.err != nil {
			return styles.AppContainer.Render(
				styles.ErrorText.Render(fmt.Sprintf("Error: %v", m.err)) + "\n\n" +
					"Press q to quit.",
			)
		}
		scanDirs := "(none)"
		if len(m.cfg.ScanDirs) > 0 {
			scanDirs = m.cfg.ScanDirs[0]
		}
		return styles.AppContainer.Render(
			styles.Title.Render(" bay setup complete!") + "\n\n" +
				styles.SuccessText.Render("Config saved to ~/.bay/config.yaml") + "\n\n" +
				fmt.Sprintf("  Scan dirs:   %s\n", scanDirs) +
				fmt.Sprintf("  Worktrees:   %s\n", m.cfg.Defaults.WorktreeLocation) +
				fmt.Sprintf("  Shell:       %s\n", m.cfg.Defaults.Shell) +
			fmt.Sprintf("  Agent:       %s\n", m.cfg.Defaults.Agent) + "\n" +
				styles.HelpBar.Render("Press Enter to continue to bay"),
		)
	}

	return ""
}
