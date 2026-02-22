package tree

// NodeType indicates whether a tree node is a repo or a session.
type NodeType int

const (
	RepoNode    NodeType = iota
	SessionNode
)

// Node is a single item in the tree.
type Node struct {
	Type        NodeType
	Name        string
	RepoName    string // for session nodes, the parent repo
	SessionName string // for session nodes, the session name
	Active      bool   // is this the active tmux session?
	Expanded    bool   // for repo nodes
	Children    []Node // session nodes under a repo
}

// Model is the tree widget state.
type Model struct {
	Nodes    []Node
	Cursor   int
	flatList []flatNode // flattened visible nodes for navigation
}

type flatNode struct {
	nodeIdx  int // index into Nodes
	childIdx int // -1 for repo, index into Children for session
}

// New creates a new tree model.
func New() Model {
	return Model{}
}

// SetNodes replaces the tree contents and rebuilds the flat list.
func (m *Model) SetNodes(nodes []Node) {
	m.Nodes = nodes
	m.rebuildFlat()
	if m.Cursor >= len(m.flatList) {
		m.Cursor = max(0, len(m.flatList)-1)
	}
}

// rebuildFlat builds the flat navigation list from visible nodes.
func (m *Model) rebuildFlat() {
	m.flatList = nil
	for i, n := range m.Nodes {
		m.flatList = append(m.flatList, flatNode{nodeIdx: i, childIdx: -1})
		if n.Expanded {
			for j := range n.Children {
				m.flatList = append(m.flatList, flatNode{nodeIdx: i, childIdx: j})
			}
		}
	}
}

// MoveUp moves the cursor up.
func (m *Model) MoveUp() {
	if m.Cursor > 0 {
		m.Cursor--
	}
}

// MoveDown moves the cursor down.
func (m *Model) MoveDown() {
	if m.Cursor < len(m.flatList)-1 {
		m.Cursor++
	}
}

// Toggle expands/collapses the current repo node.
func (m *Model) Toggle() {
	if len(m.flatList) == 0 {
		return
	}
	fn := m.flatList[m.Cursor]
	if fn.childIdx == -1 {
		// It's a repo node
		m.Nodes[fn.nodeIdx].Expanded = !m.Nodes[fn.nodeIdx].Expanded
		m.rebuildFlat()
		// Keep cursor in bounds
		if m.Cursor >= len(m.flatList) {
			m.Cursor = len(m.flatList) - 1
		}
	}
}

// Selected returns info about the currently selected node.
// Returns (nodeType, repoName, sessionName).
func (m *Model) Selected() (NodeType, string, string) {
	if len(m.flatList) == 0 {
		return RepoNode, "", ""
	}
	fn := m.flatList[m.Cursor]
	node := m.Nodes[fn.nodeIdx]
	if fn.childIdx == -1 {
		return RepoNode, node.Name, ""
	}
	child := node.Children[fn.childIdx]
	return SessionNode, node.Name, child.SessionName
}

// SelectedRepoName returns the repo name of the currently selected item.
func (m *Model) SelectedRepoName() string {
	if len(m.flatList) == 0 {
		return ""
	}
	fn := m.flatList[m.Cursor]
	return m.Nodes[fn.nodeIdx].Name
}

// FlatLen returns the number of visible items.
func (m *Model) FlatLen() int {
	return len(m.flatList)
}

// CursorFlat returns the flat node at the cursor.
func (m *Model) CursorFlat() *flatNode {
	if m.Cursor < 0 || m.Cursor >= len(m.flatList) {
		return nil
	}
	return &m.flatList[m.Cursor]
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
