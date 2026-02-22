package create

import (
	"fmt"
	"strings"

	"github.com/anawarkar/bay/internal/tui/styles"
)

// View renders the session creation flow.
func (m Model) View() string {
	switch m.step {
	case stepPickRepo:
		return m.viewPickRepo()
	case stepWorktreeChoice:
		return m.viewWorktreeChoice()
	case stepBranchName:
		return m.viewBranchName()
	case stepSessionName:
		return m.viewSessionName()
	case stepCreating:
		return styles.AppContainer.Render(
			styles.Title.Render(" new session") + "\n\n" +
				"Creating session...\n",
		)
	case stepCreated:
		return m.viewCreated()
	}
	return ""
}

func (m Model) viewPickRepo() string {
	var b strings.Builder
	b.WriteString(styles.Title.Render(" new session") + "\n\n")
	b.WriteString(styles.InputLabel.Render("Select a repository:") + "\n\n")

	for i, repo := range m.repos {
		cursor := "  "
		if i == m.repoCursor {
			cursor = "> "
			b.WriteString(styles.SelectedLine.Render(cursor + repo.Name))
		} else {
			b.WriteString(styles.SessionName.Render(cursor + repo.Name))
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(styles.HelpBar.Render("j/k navigate, Enter select, Esc cancel"))

	return styles.AppContainer.Render(b.String())
}

func (m Model) viewWorktreeChoice() string {
	return styles.AppContainer.Render(
		styles.Title.Render(" new session") + "\n\n" +
			styles.InputLabel.Render(fmt.Sprintf("Repo: %s", m.selectedRepo.Name)) + "\n\n" +
			"How do you want to work?\n\n" +
			"  1. Main repo directory\n" +
			"  2. New git worktree (separate branch)\n\n" +
			styles.HelpBar.Render("Press 1 or 2, Esc cancel"),
	)
}

func (m Model) viewBranchName() string {
	return styles.AppContainer.Render(
		styles.Title.Render(" new session") + "\n\n" +
			styles.InputLabel.Render(fmt.Sprintf("Repo: %s", m.selectedRepo.Name)) + "\n\n" +
			"Enter branch name for worktree:\n\n" +
			m.branchInput.View() + "\n\n" +
			styles.HelpBar.Render("Enter confirm, Esc cancel"),
	)
}

func (m Model) viewSessionName() string {
	return styles.AppContainer.Render(
		styles.Title.Render(" new session") + "\n\n" +
			styles.InputLabel.Render(fmt.Sprintf("Repo: %s", m.selectedRepo.Name)) + "\n\n" +
			"Session name:\n\n" +
			m.nameInput.View() + "\n\n" +
			styles.HelpBar.Render("Enter confirm, Esc cancel"),
	)
}

func (m Model) viewCreated() string {
	if m.err != nil {
		return styles.AppContainer.Render(
			styles.Title.Render(" new session") + "\n\n" +
				styles.ErrorText.Render(fmt.Sprintf("Error: %v", m.err)) + "\n\n" +
				styles.HelpBar.Render("Press Enter to go back"),
		)
	}
	return styles.AppContainer.Render(
		styles.Title.Render(" new session") + "\n\n" +
			styles.SuccessText.Render(fmt.Sprintf("Session '%s' created!", m.created.Name)) + "\n\n" +
			fmt.Sprintf("  Repo:    %s\n", m.created.Repo) +
			fmt.Sprintf("  Dir:     %s\n", m.created.WorkingDir) + "\n" +
			styles.HelpBar.Render("Press Enter to continue"),
	)
}
