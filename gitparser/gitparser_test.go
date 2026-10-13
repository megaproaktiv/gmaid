package gitparser

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/megaproaktiv/gmaid/gitmodel"
)

// TestParseAllTestdata parses every gitGraph file in ../testdata and checks the
// result is structurally sane.
func TestParseAllTestdata(t *testing.T) {
	files, _ := filepath.Glob("../testdata/*.mmd")
	n := 0
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		if !isGitGraph(string(b)) {
			continue
		}
		n++
		g, err := Parse(string(b))
		if err != nil {
			t.Fatalf("parse %s: %v", f, err)
		}
		if len(g.Branches) == 0 {
			t.Errorf("%s: no branches", f)
		}
		if len(g.Commits) == 0 {
			t.Errorf("%s: no commits", f)
		}
		if g.Branches[0].Name != g.MainBranch {
			t.Errorf("%s: first branch %q != main %q", f, g.Branches[0].Name, g.MainBranch)
		}
	}
	if n == 0 {
		t.Skip("no gitGraph testdata found")
	}
}

// TestParseGit100 asserts the exact structure of git-100.mmd.
func TestParseGit100(t *testing.T) {
	b, err := os.ReadFile("../testdata/git-100.mmd")
	if err != nil {
		t.Skip("git-100.mmd not present")
	}
	g, err := Parse(string(b))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	wantBranches := []string{"main", "hot", "extraordinary1", "extraordinary"}
	if len(g.Branches) != len(wantBranches) {
		t.Fatalf("branches = %d, want %d", len(g.Branches), len(wantBranches))
	}
	for i, name := range wantBranches {
		if g.Branches[i].Name != name {
			t.Errorf("branch[%d] = %q, want %q", i, g.Branches[i].Name, name)
		}
		if g.Branches[i].Index != i {
			t.Errorf("branch[%d] index = %d, want %d", i, g.Branches[i].Index, i)
		}
	}

	if len(g.Commits) != 9 {
		t.Fatalf("commits = %d, want 9", len(g.Commits))
	}

	// First commit: id "clan", tag v0.1.0, on main, explicit id.
	c0 := g.Commits[0]
	if c0.ID != "clan" || c0.Tag != "v0.1.0" || c0.Branch != "main" || c0.AutoID {
		t.Errorf("commit0 = %+v, want id=clan tag=v0.1.0 branch=main autoID=false", c0)
	}

	// Merge commits at seq 4, 7, 8; all auto-id, kind merge.
	for _, seq := range []int{4, 7, 8} {
		c := g.Commits[seq]
		if c.Kind != gitmodel.CommitMerge {
			t.Errorf("commit seq %d kind = %d, want merge(%d)", seq, c.Kind, gitmodel.CommitMerge)
		}
		if !c.AutoID {
			t.Errorf("merge commit seq %d should have AutoID=true", seq)
		}
		if c.MergeFrom < 0 {
			t.Errorf("merge commit seq %d has no MergeFrom", seq)
		}
	}

	// Final merge carries tag v0.2.0 on main.
	last := g.Commits[8]
	if last.Tag != "v0.2.0" || last.Branch != "main" {
		t.Errorf("last commit = %+v, want tag=v0.2.0 branch=main", last)
	}
}

// TestParseOptions checks id/tag/type extraction, quoted and bare.
func TestParseOptions(t *testing.T) {
	src := `gitGraph
   commit id: "a b c" tag: "v1" type: HIGHLIGHT
   commit
`
	g, err := Parse(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(g.Commits) != 2 {
		t.Fatalf("commits = %d, want 2", len(g.Commits))
	}
	if g.Commits[0].ID != "a b c" {
		t.Errorf("id = %q, want %q", g.Commits[0].ID, "a b c")
	}
	if g.Commits[0].Tag != "v1" {
		t.Errorf("tag = %q, want v1", g.Commits[0].Tag)
	}
	if g.Commits[0].Kind != gitmodel.CommitHighlight {
		t.Errorf("kind = %d, want highlight(%d)", g.Commits[0].Kind, gitmodel.CommitHighlight)
	}
	if !g.Commits[1].AutoID {
		t.Errorf("bare commit should have AutoID=true")
	}
}

func isGitGraph(src string) bool {
	for _, line := range splitLines(src) {
		if line == "" {
			continue
		}
		return len(line) >= 8 && line[:8] == "gitGraph"
	}
	return false
}

func splitLines(s string) []string {
	var out []string
	cur := ""
	for _, r := range s {
		if r == '\n' {
			out = append(out, trimSpace(cur))
			cur = ""
			continue
		}
		cur += string(r)
	}
	out = append(out, trimSpace(cur))
	return out
}

func trimSpace(s string) string {
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\r') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}
