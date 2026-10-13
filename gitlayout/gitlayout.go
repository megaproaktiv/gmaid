// Package gitlayout computes pixel geometry for a parsed gitGraph: one
// horizontal lane (row) per branch in creation order, commits placed left to
// right by their global chronological sequence, plus the connectors that link
// each commit to its parent and (for merges) to the merged-in branch tip.
package gitlayout

import (
	"github.com/megaproaktiv/gmaid/gitmodel"
	"github.com/megaproaktiv/gmaid/textmetrics"
)

// Config holds spacing parameters (pixels, pre-scale), tuned mermaid-like.
type Config struct {
	Margin      float64 // outer margin
	LabelBoxW   float64 // reserved width on the left for branch label boxes
	LabelBoxH   float64 // branch label box height
	LabelPadX   float64 // horizontal text padding inside a label box
	LaneGap     float64 // vertical distance between branch lanes
	CommitSep   float64 // horizontal distance between successive commits (by seq)
	FirstCommit float64 // x gap from label area to the first commit
	TopPad      float64 // space above the first lane (for tags)
	BottomPad   float64 // space below the last lane (for rotated commit ids)
	Radius      float64 // commit circle radius
	FontSize    float64 // commit id font
	LabelFont   float64 // branch label font
	TagFont     float64 // tag font
}

// DefaultConfig returns mermaid-like defaults (tuned against reference-git).
func DefaultConfig() Config {
	return Config{
		Margin:      10,
		LabelBoxW:   118,
		LabelBoxH:   28,
		LabelPadX:   12,
		LaneGap:     87,
		CommitSep:   49,
		FirstCommit: 36,
		TopPad:      34,
		BottomPad:   82,
		Radius:      9,
		FontSize:    13,
		LabelFont:   15,
		TagFont:     12,
	}
}

// Point is a 2D coordinate.
type Point struct{ X, Y float64 }

// CommitDot is a positioned commit.
type CommitDot struct {
	Commit  *gitmodel.Commit
	X, Y    float64
	Branch  int // branch index (for color)
	Kind    gitmodel.CommitKind
	ID      string
	ShowID  bool // false for auto-generated ids (e.g. merge commits)
	Tag     string
	TagX    float64
	TagY    float64
}

// LaneLabel is a branch's colored label box on the left.
type LaneLabel struct {
	Name       string
	Index      int
	X, Y, W, H float64
	CY         float64 // lane center y
}

// Connector links a commit to a parent (or merge source). It is an orthogonal/
// curved path from (X0,Y0) to (X1,Y1); Color follows the branch that "owns" it.
type Connector struct {
	X0, Y0, X1, Y1 float64
	ColorIndex     int  // branch index whose color to use
	Merge          bool // merge connector (drawn in the merged-in branch color)
}

// Diagram is the fully positioned gitGraph.
type Diagram struct {
	Dots       []CommitDot
	Labels     []LaneLabel
	Connectors []Connector
	Width      float64
	Height     float64
	NumBranch  int
}

// Engine lays out gitGraph diagrams.
type Engine struct {
	cfg   Config
	label *textmetrics.Face
	idf   *textmetrics.Face
}

// NewEngine builds an engine.
func NewEngine(cfg Config) *Engine {
	return &Engine{
		cfg:   cfg,
		label: textmetrics.New(cfg.LabelFont, true),
		idf:   textmetrics.New(cfg.FontSize, false),
	}
}

// Layout computes geometry for the whole graph.
func (e *Engine) Layout(g *gitmodel.Graph) *Diagram {
	cfg := e.cfg
	out := &Diagram{NumBranch: len(g.Branches)}

	if len(g.Commits) == 0 || len(g.Branches) == 0 {
		out.Width, out.Height = 1, 1
		return out
	}

	// Widest branch label determines the label-box width (floored at LabelBoxW).
	labelW := cfg.LabelBoxW
	for _, b := range g.Branches {
		w, _ := e.label.Measure(b.Name)
		if need := w + 2*cfg.LabelPadX; need > labelW {
			labelW = need
		}
	}

	// Lane geometry.
	laneY := func(order int) float64 {
		return cfg.Margin + cfg.TopPad + float64(order)*cfg.LaneGap
	}
	// Commit x by global sequence.
	chartLeft := cfg.Margin + labelW + cfg.FirstCommit
	commitX := func(seq int) float64 {
		return chartLeft + float64(seq)*cfg.CommitSep
	}

	// Branch lane labels (left column).
	for _, b := range g.Branches {
		cy := laneY(b.Order)
		out.Labels = append(out.Labels, LaneLabel{
			Name: b.Name, Index: b.Index,
			X: cfg.Margin, Y: cy - cfg.LabelBoxH/2,
			W: labelW, H: cfg.LabelBoxH, CY: cy,
		})
	}

	// Position commits.
	dotByIdx := make([]int, len(g.Commits)) // commit index -> dot index
	for i, c := range g.Commits {
		br := g.Branch(c.Branch)
		order := 0
		idx := 0
		if br != nil {
			order = br.Order
			idx = br.Index
		}
		d := CommitDot{
			Commit: c, X: commitX(c.Seq), Y: laneY(order),
			Branch: idx, Kind: c.Kind, ID: c.ID, ShowID: !c.AutoID, Tag: c.Tag,
		}
		if c.Tag != "" {
			d.TagX = d.X
			d.TagY = d.Y - cfg.LabelBoxH
		}
		dotByIdx[i] = len(out.Dots)
		out.Dots = append(out.Dots, d)
	}

	// Connectors: each commit to its primary parent, and merges to their source.
	for i, c := range g.Commits {
		d := out.Dots[dotByIdx[i]]
		if c.Parent >= 0 {
			p := out.Dots[dotByIdx[c.Parent]]
			out.Connectors = append(out.Connectors, Connector{
				X0: p.X, Y0: p.Y, X1: d.X, Y1: d.Y,
				ColorIndex: d.Branch,
			})
		}
		if c.MergeFrom >= 0 {
			s := out.Dots[dotByIdx[c.MergeFrom]]
			out.Connectors = append(out.Connectors, Connector{
				X0: s.X, Y0: s.Y, X1: d.X, Y1: d.Y,
				ColorIndex: s.Branch, Merge: true,
			})
		}
	}

	// Canvas size.
	maxSeq := 0
	for _, c := range g.Commits {
		if c.Seq > maxSeq {
			maxSeq = c.Seq
		}
	}
	rightPad := cfg.CommitSep // room for the last tag/id
	out.Width = commitX(maxSeq) + rightPad + cfg.Margin
	lastLaneY := laneY(len(g.Branches) - 1)
	out.Height = lastLaneY + cfg.BottomPad + cfg.Margin
	return out
}
