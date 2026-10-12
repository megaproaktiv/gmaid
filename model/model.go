// Package model defines the in-memory representation of a parsed mermaid
// flowchart: the graph, its nodes, edges, subgraphs and styling.
package model

// Direction is the flow orientation of a (sub)graph.
type Direction string

const (
	DirTB Direction = "TB" // top to bottom (TD is an alias)
	DirLR Direction = "LR" // left to right
	DirBT Direction = "BT" // bottom to top
	DirRL Direction = "RL" // right to left
)

// Shape enumerates the node geometries we support.
type Shape int

const (
	ShapeRect             Shape = iota
	ShapeRound                  // (text)      rounded rectangle
	ShapeStadium                // ([text])    pill
	ShapeSubroutine             // [[text]]    rectangle with side bars
	ShapeCylinder               // [(text)]    database
	ShapeCircle                 // ((text))
	ShapeDiamond                // {text}      decision
	ShapeHexagon                // {{text}}
	ShapeParallelogram          // [/text/]
	ShapeParallelogramAlt       // [\text\]
	ShapeTrapezoid              // [/text\]
	ShapeTrapezoidAlt           // [\text/]
)

// EdgeLineStyle is the stroke style of an edge.
type EdgeLineStyle int

const (
	LineSolid EdgeLineStyle = iota
	LineDotted
	LineThick
	LineInvisible // ~~~
)

// ArrowType is the terminator drawn at the destination end.
type ArrowType int

const (
	ArrowNormal ArrowType = iota // -->
	ArrowNone                    // ---  (open link)
	ArrowCircle                  // --o
	ArrowCross                   // --x
)

// Node is a single vertex in the graph.
type Node struct {
	ID    string
	Label string // may contain '\n' for explicit line breaks
	Shape Shape
	Style *Style // optional per-node style from a `style` directive or class

	// Parent subgraph id, "" if top-level.
	Parent string
}

// Edge connects two nodes.
type Edge struct {
	From   string
	To     string
	Label  string
	Line   EdgeLineStyle
	Arrow  ArrowType
	MinLen int // requested minimum rank span (extra dashes), >=1
}

// Subgraph is a titled container that can nest nodes and other subgraphs.
type Subgraph struct {
	ID        string
	Title     string
	Direction Direction // own direction if declared, else "" meaning inherit
	Parent    string    // parent subgraph id, "" if top-level
	Children  []string  // ids of nodes and nested subgraphs, in declaration order
}

// Style holds the subset of CSS-ish properties mermaid supports via `style`.
type Style struct {
	Fill        string // hex color like #FF9900
	Stroke      string
	StrokeWidth float64 // in px, 0 means default
	Color       string  // text color
	Dashed      bool
}

// Graph is the whole parsed diagram.
type Graph struct {
	Direction Direction
	Nodes     map[string]*Node
	NodeOrder []string // insertion order for deterministic layout
	Edges     []*Edge
	Subgraphs map[string]*Subgraph
	SubOrder  []string
}

// NewGraph returns an initialized empty graph.
func NewGraph() *Graph {
	return &Graph{
		Direction: DirTB,
		Nodes:     map[string]*Node{},
		Subgraphs: map[string]*Subgraph{},
	}
}

// EnsureNode returns the node with the given id, creating a default rectangle
// node if it does not yet exist.
func (g *Graph) EnsureNode(id string) *Node {
	if n, ok := g.Nodes[id]; ok {
		return n
	}
	n := &Node{ID: id, Label: id, Shape: ShapeRect}
	g.Nodes[id] = n
	g.NodeOrder = append(g.NodeOrder, id)
	return n
}
