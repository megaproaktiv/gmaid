// Package gitparser parses Mermaid gitGraph source into a gitmodel.Graph.
//
// Supported syntax (mermaid gitGraph):
//
//	gitGraph                       (optionally "gitGraph:" or "gitGraph LR:" / "gitGraph TB:")
//	   commit                      bare commit (auto id)
//	   commit id: "abc"            with id
//	   commit tag: "v1"            with tag
//	   commit type: HIGHLIGHT      with type NORMAL|REVERSE|HIGHLIGHT
//	   branch NAME                 create branch
//	   checkout NAME / switch NAME current branch
//	   merge NAME [tag: "..."] [type: ...]
//	   cherry-pick id: "..."
//	   %% comment
//
// Front-matter and %%{init}%% directives are skipped. Unknown lines are ignored.
package gitparser

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/megaproaktiv/gmaid/gitmodel"
)

// Parse converts gitGraph source text into a Graph.
func Parse(src string) (*gitmodel.Graph, error) {
	lines := strings.Split(src, "\n")

	mainName := "main"
	g := gitmodel.New(mainName)

	current := mainName // currently checked-out branch
	seq := 0
	autoCount := 0
	headerSeen := false

	// branchTip[name] = index into g.Commits of the latest commit on that
	// branch, or -1 if the branch has no commits yet.
	branchTip := map[string]int{mainName: -1}

	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "%%") || strings.HasPrefix(line, "---") {
			continue
		}

		fields := strings.Fields(line)
		kw := fields[0]

		if !headerSeen {
			if strings.HasPrefix(kw, "gitGraph") {
				headerSeen = true
				continue
			}
			return nil, fmt.Errorf("not a gitGraph diagram (first keyword %q)", kw)
		}

		switch kw {
		case "commit":
			id, tag, typ := parseCommitOpts(line)
			auto := false
			if id == "" {
				autoCount++
				id = strconv.Itoa(autoCount - 1)
				auto = true
			}
			c := &gitmodel.Commit{
				ID: id, AutoID: auto, Tag: tag, Kind: commitKind(typ),
				Branch: current, Seq: seq, Parent: branchTip[current], MergeFrom: -1,
			}
			idx := len(g.Commits)
			g.Commits = append(g.Commits, c)
			branchTip[current] = idx
			seq++

		case "branch":
			if len(fields) < 2 {
				continue
			}
			name := unquote(fields[1])
			g.AddBranch(name)
			branchTip[name] = branchTip[current] // fork from current tip

		case "checkout", "switch":
			if len(fields) < 2 {
				continue
			}
			name := unquote(fields[1])
			if !g.HasBranch(name) {
				g.AddBranch(name)
				branchTip[name] = branchTip[current]
			}
			current = name

		case "merge":
			if len(fields) < 2 {
				continue
			}
			from := unquote(fields[1])
			_, tag, typ := parseCommitOpts(line)
			kind := gitmodel.CommitMerge
			if k := commitKind(typ); k != gitmodel.CommitNormal {
				kind = k
			}
			autoCount++
			c := &gitmodel.Commit{
				ID: strconv.Itoa(autoCount - 1), AutoID: true, Tag: tag, Kind: kind,
				Branch: current, Seq: seq, Parent: branchTip[current],
				MergeFrom: branchTip[from],
			}
			idx := len(g.Commits)
			g.Commits = append(g.Commits, c)
			branchTip[current] = idx
			seq++

		case "cherry-pick":
			id, tag, _ := parseCommitOpts(line)
			auto := false
			if id == "" {
				autoCount++
				id = strconv.Itoa(autoCount - 1)
				auto = true
			}
			c := &gitmodel.Commit{
				ID: id, AutoID: auto, Tag: tag, Kind: gitmodel.CommitCherryPick,
				Branch: current, Seq: seq, Parent: branchTip[current], MergeFrom: -1,
			}
			idx := len(g.Commits)
			g.Commits = append(g.Commits, c)
			branchTip[current] = idx
			seq++

		default:
			// ignore unknown lines (accTitle, options, etc.)
		}
	}

	if !headerSeen {
		return nil, fmt.Errorf("empty or non-gitGraph input")
	}
	return g, nil
}

// parseCommitOpts extracts id:, tag: and type: values from a commit/merge line.
func parseCommitOpts(line string) (id, tag, typ string) {
	id = extractOpt(line, "id:")
	tag = extractOpt(line, "tag:")
	typ = extractOpt(line, "type:")
	return
}

// extractOpt finds key in the line and returns its value, which may be a
// quoted string ("...") or a bare token (used for type:).
func extractOpt(line, key string) string {
	i := strings.Index(line, key)
	if i < 0 {
		return ""
	}
	rest := strings.TrimSpace(line[i+len(key):])
	if rest == "" {
		return ""
	}
	if rest[0] == '"' {
		if j := strings.IndexByte(rest[1:], '"'); j >= 0 {
			return rest[1 : 1+j]
		}
		return strings.Trim(rest, `"`)
	}
	return strings.Fields(rest)[0]
}

// commitKind maps a mermaid type: token to a CommitKind.
func commitKind(typ string) gitmodel.CommitKind {
	switch strings.ToUpper(strings.TrimSpace(typ)) {
	case "REVERSE":
		return gitmodel.CommitReverse
	case "HIGHLIGHT":
		return gitmodel.CommitHighlight
	default:
		return gitmodel.CommitNormal
	}
}

// unquote strips surrounding double quotes from a token.
func unquote(s string) string { return strings.Trim(s, `"`) }
