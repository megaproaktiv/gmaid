// Package render rasterizes a laid-out Diagram to a PNG using the pure-Go gg
// drawing library. Styling approximates mermaid's default ("neo"/redux) look:
// light node fills, dark strokes, labels centered, subgraph containers with a
// title, and support for per-node `style` fill/stroke/color directives.
package render

import (
	"image/color"

	"github.com/fogleman/gg"

	"github.com/megaproaktiv/gmaid/layout"
	"github.com/megaproaktiv/gmaid/textmetrics"
)

// Theme holds the default palette used when a node has no explicit style.
type Theme struct {
	Background color.Color
	NodeFill   color.Color
	NodeStroke color.Color
	NodeText   color.Color
	EdgeStroke color.Color
	EdgeText   color.Color
	LabelBG    color.Color
	SubFill    []color.Color // per-depth subgraph fills
	SubStroke  color.Color
	SubTitle   color.Color
}

// DefaultTheme returns a mermaid-like default theme.
func DefaultTheme() Theme {
	return Theme{
		Background: color.White,
		NodeFill:   hex("#ECECFF"),
		NodeStroke: hex("#9370DB"),
		NodeText:   hex("#1A1A1A"),
		EdgeStroke: hex("#333333"),
		EdgeText:   hex("#333333"),
		LabelBG:    hex("#FFFFFF"),
		SubFill: []color.Color{
			hex("#F4F4FF"),
			hex("#ECECFF"),
			hex("#E2E2FA"),
		},
		SubStroke: hex("#9370DB"),
		SubTitle:  hex("#1A1A1A"),
	}
}

// Renderer draws diagrams.
type Renderer struct {
	theme Theme
	cfg   layout.Config
	scale float64
	face  *textmetrics.Face
	bold  *textmetrics.Face
}

// New returns a renderer. scale > 1 produces a higher-resolution image.
func New(theme Theme, cfg layout.Config, scale float64) *Renderer {
	if scale <= 0 {
		scale = 1
	}
	return &Renderer{
		theme: theme,
		cfg:   cfg,
		scale: scale,
		face:  textmetrics.New(cfg.FontSize, false),
		bold:  textmetrics.New(cfg.FontSize, true),
	}
}

// Render draws the diagram and writes a PNG to path.
func (r *Renderer) Render(dia *layout.Diagram, path string) error {
	w := int(dia.Width*r.scale + 0.5)
	h := int(dia.Height*r.scale + 0.5)
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	dc := gg.NewContext(w, h)
	dc.Scale(r.scale, r.scale)

	// background
	dc.SetColor(r.theme.Background)
	dc.Clear()

	// subgraph containers first (behind nodes). Draw shallow depth first so
	// nested ones layer on top.
	subs := append([]layout.SubBox(nil), dia.Subs...)
	sortSubsByDepth(subs)
	for _, s := range subs {
		r.drawSubgraph(dc, s)
	}

	// edges (behind nodes so arrowheads tuck under borders cleanly)
	for _, e := range dia.Edges {
		r.drawEdge(dc, e)
	}

	// nodes
	for _, n := range dia.Nodes {
		r.drawNode(dc, n)
	}

	// edge labels on top of edges
	for _, e := range dia.Edges {
		if e.Label != "" {
			r.drawEdgeLabel(dc, e)
		}
	}

	return dc.SavePNG(path)
}

func sortSubsByDepth(subs []layout.SubBox) {
	// simple insertion sort by Depth (small slices)
	for i := 1; i < len(subs); i++ {
		for j := i; j > 0 && subs[j-1].Depth > subs[j].Depth; j-- {
			subs[j-1], subs[j] = subs[j], subs[j-1]
		}
	}
}

// hex parses a #rrggbb or #rgb string into a color.
func hex(s string) color.Color {
	c, ok := parseHex(s)
	if !ok {
		return color.Black
	}
	return c
}
