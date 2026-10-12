// Package layout computes geometric positions for a parsed flowchart using a
// Sugiyama-style layered algorithm. Subgraphs are laid out recursively and
// embedded as composite blocks in their parent, so nested directions and
// containment are respected.
package layout

import (
	"gmaid/model"
)

// Config holds spacing and sizing parameters (in pixels, pre-scale).
type Config struct {
	NodeHPad   float64 // horizontal text padding inside a node
	NodeVPad   float64 // vertical text padding inside a node
	MinNodeW   float64
	MinNodeH   float64
	RankSep    float64 // gap between consecutive ranks (along flow axis)
	NodeSep    float64 // gap between nodes in the same rank
	SubPad     float64 // padding inside a subgraph box
	SubTitleH  float64 // vertical space reserved for a subgraph title
	FontSize   float64
	LineHeight float64 // per text line
	CharW      float64 // approx average glyph width at FontSize (fallback)
	Margin     float64 // outer canvas margin
}

// DefaultConfig returns sensible defaults tuned to look mermaid-like.
func DefaultConfig() Config {
	return Config{
		NodeHPad:   18,
		NodeVPad:   9,
		MinNodeW:   40,
		MinNodeH:   36,
		RankSep:    105,
		NodeSep:    28,
		SubPad:     22,
		SubTitleH:  26,
		FontSize:   15,
		LineHeight: 19,
		CharW:      8.2,
		Margin:     24,
	}
}

// Point is a 2D coordinate.
type Point struct{ X, Y float64 }

// NodeBox is the final placement of a node.
type NodeBox struct {
	ID    string
	Node  *model.Node
	X, Y  float64 // top-left
	W, H  float64
	Lines []string // label split into display lines
}

// Center returns the box center.
func (b NodeBox) Center() Point { return Point{b.X + b.W/2, b.Y + b.H/2} }

// SubBox is the final placement of a subgraph container.
type SubBox struct {
	ID    string
	Title string
	X, Y  float64
	W, H  float64
	Depth int // nesting depth for shading
}

// EdgePath is the routed polyline for an edge, in absolute coords.
type EdgePath struct {
	Edge     *model.Edge
	Points   []Point
	Label    string
	LabelPos Point
}

// Diagram is the complete positioned layout ready for rendering.
type Diagram struct {
	Nodes  []NodeBox
	Subs   []SubBox
	Edges  []EdgePath
	Width  float64
	Height float64
}
