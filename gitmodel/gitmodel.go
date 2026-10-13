// Package gitmodel defines the in-memory representation of a parsed Mermaid
// gitGraph diagram: an ordered list of operations distilled into branches,
// commits (including merge and cherry-pick commits) and tags.
//
// Separate from the flowchart, sequence and gantt models so gitGraph support
// evolves independently.
package gitmodel

// CommitKind distinguishes normal commits from merge and cherry-pick commits.
type CommitKind int

const (
	// CommitNormal is an ordinary commit (filled circle).
	CommitNormal CommitKind = iota
	// CommitMerge is a merge commit (hollow/ringed circle).
	CommitMerge
	// CommitReverse is a commit flagged type: REVERSE (crossed circle).
	CommitReverse
	// CommitHighlight is a commit flagged type: HIGHLIGHT (square).
	CommitHighlight
	// CommitCherryPick is a cherry-picked commit.
	CommitCherryPick
)

// Branch is a line of development. Index is its creation order (0 = main),
// which drives both its lane (row) and its color.
type Branch struct {
	Name  string
	Index int
	Order int // vertical lane order (creation order; main first)
}

// Commit is a single node on the graph.
type Commit struct {
	ID       string     // commit id (explicit or auto-generated)
	AutoID   bool       // true when ID was auto-generated (not drawn as a label)
	Tag      string     // optional tag label
	Kind     CommitKind // normal / merge / reverse / highlight / cherry-pick
	Branch   string     // branch this commit lives on
	Seq      int        // global chronological order (x position index)
	Parent   int        // index into Commits of the primary parent (-1 if none)
	MergeFrom int       // for merge commits: index into Commits of the merged-in tip (-1 otherwise)
}

// Graph is a whole parsed gitGraph diagram.
type Graph struct {
	Branches   []*Branch
	Commits    []*Commit
	MainBranch string // name of the initial branch (default "main")

	byName map[string]*Branch
}

// New returns an empty graph with the default main branch registered.
func New(mainName string) *Graph {
	if mainName == "" {
		mainName = "main"
	}
	g := &Graph{MainBranch: mainName, byName: map[string]*Branch{}}
	g.AddBranch(mainName)
	return g
}

// AddBranch registers a branch by name if not already present and returns it.
func (g *Graph) AddBranch(name string) *Branch {
	if b, ok := g.byName[name]; ok {
		return b
	}
	b := &Branch{Name: name, Index: len(g.Branches), Order: len(g.Branches)}
	g.Branches = append(g.Branches, b)
	g.byName[name] = b
	return b
}

// Branch returns a registered branch by name (nil if unknown).
func (g *Graph) Branch(name string) *Branch {
	return g.byName[name]
}

// HasBranch reports whether a branch name is registered.
func (g *Graph) HasBranch(name string) bool {
	_, ok := g.byName[name]
	return ok
}
