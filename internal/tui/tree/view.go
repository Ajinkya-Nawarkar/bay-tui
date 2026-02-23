package tree

import (
	"fmt"
	"strings"

	"bay/internal/tui/styles"
)

// View renders the tree widget.
func (m *Model) View() string {
	if len(m.Nodes) == 0 {
		return styles.NoSessions.Render("No repos found.\nPress 's' to configure.")
	}

	var b strings.Builder
	flatIdx := 0

	for i, node := range m.Nodes {
		// Repo header
		icon := "▸"
		if node.Expanded {
			icon = "▾"
		}
		line := fmt.Sprintf("%s %s", icon, node.Name)

		if flatIdx == m.Cursor {
			b.WriteString(styles.SelectedLine.Render(line))
		} else {
			b.WriteString(styles.RepoName.Render(line))
		}
		b.WriteString("\n")
		flatIdx++

		if node.Expanded {
			if len(node.Children) == 0 {
				b.WriteString(styles.NoSessions.Render("  (empty)"))
				b.WriteString("\n")
			}
			for j, child := range node.Children {
				connector := "├─"
				if j == len(m.Nodes[i].Children)-1 {
					connector = "└─"
				}
				name := child.Name
				suffix := ""
				if child.Active {
					suffix = " ●"
				}
				sessionLine := fmt.Sprintf("  %s %s%s", connector, name, suffix)

				if flatIdx == m.Cursor {
					b.WriteString(styles.SelectedLine.Render(sessionLine))
				} else if child.Active {
					b.WriteString(styles.ActiveSession.Render(sessionLine))
				} else {
					b.WriteString(styles.SessionName.Render(sessionLine))
				}
				b.WriteString("\n")
				flatIdx++
			}
		}
	}

	// Trim trailing newline
	result := b.String()
	return strings.TrimRight(result, "\n")
}
