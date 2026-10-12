package render

import (
	"math"

	"github.com/fogleman/gg"

	"gmaid/layout"
	"gmaid/model"
)

// drawSubgraph renders a subgraph container box with its title.
func (r *Renderer) drawSubgraph(dc *gg.Context, s layout.SubBox) {
	fill := r.theme.SubFill[s.Depth%len(r.theme.SubFill)]
	r.roundRect(dc, s.X, s.Y, s.W, s.H, 6)
	dc.SetColor(fill)
	dc.FillPreserve()
	dc.SetColor(r.theme.SubStroke)
	dc.SetLineWidth(1.3)
	dc.SetDash() // solid
	dc.Stroke()

	if s.Title != "" {
		dc.SetFontFace(r.bold.FontFace())
		dc.SetColor(r.theme.SubTitle)
		tx := s.X + s.W/2
		ty := s.Y + 8 + r.cfg.FontSize*0.5
		dc.DrawStringAnchored(s.Title, tx, ty, 0.5, 0.5)
	}
}

// drawNode renders a single node with its shape, fill, border and label.
func (r *Renderer) drawNode(dc *gg.Context, n layout.NodeBox) {
	fill := r.theme.NodeFill
	stroke := r.theme.NodeStroke
	textCol := r.theme.NodeText
	lineW := 1.3

	if st := n.Node.Style; st != nil {
		if c, ok := parseHex(st.Fill); ok {
			fill = c
		}
		if c, ok := parseHex(st.Stroke); ok {
			stroke = c
		}
		if c, ok := parseHex(st.Color); ok {
			textCol = c
		}
		if st.StrokeWidth > 0 {
			lineW = st.StrokeWidth
		}
	}

	r.nodePath(dc, n)
	dc.SetColor(fill)
	dc.FillPreserve()
	dc.SetColor(stroke)
	dc.SetLineWidth(lineW)
	dc.SetDash()
	dc.Stroke()

	// label
	dc.SetFontFace(r.face.FontFace())
	dc.SetColor(textCol)
	cx := n.X + n.W/2
	cy := n.Y + n.H/2
	total := float64(len(n.Lines)) * r.cfg.LineHeight
	startY := cy - total/2 + r.cfg.LineHeight/2
	for i, ln := range n.Lines {
		dc.DrawStringAnchored(ln, cx, startY+float64(i)*r.cfg.LineHeight, 0.5, 0.5)
	}
}

// nodePath builds the outline path for a node's shape (left on the context,
// not yet filled or stroked).
func (r *Renderer) nodePath(dc *gg.Context, n layout.NodeBox) {
	x, y, w, h := n.X, n.Y, n.W, n.H
	switch n.Node.Shape {
	case model.ShapeRound:
		r.roundRect(dc, x, y, w, h, 10)
	case model.ShapeStadium:
		r.roundRect(dc, x, y, w, h, h/2)
	case model.ShapeCircle:
		dc.DrawEllipse(x+w/2, y+h/2, w/2, h/2)
	case model.ShapeDiamond:
		dc.MoveTo(x+w/2, y)
		dc.LineTo(x+w, y+h/2)
		dc.LineTo(x+w/2, y+h)
		dc.LineTo(x, y+h/2)
		dc.ClosePath()
	case model.ShapeHexagon:
		inset := w * 0.15
		dc.MoveTo(x+inset, y)
		dc.LineTo(x+w-inset, y)
		dc.LineTo(x+w, y+h/2)
		dc.LineTo(x+w-inset, y+h)
		dc.LineTo(x+inset, y+h)
		dc.LineTo(x, y+h/2)
		dc.ClosePath()
	case model.ShapeParallelogram:
		sk := w * 0.18
		dc.MoveTo(x+sk, y)
		dc.LineTo(x+w, y)
		dc.LineTo(x+w-sk, y+h)
		dc.LineTo(x, y+h)
		dc.ClosePath()
	case model.ShapeParallelogramAlt:
		sk := w * 0.18
		dc.MoveTo(x, y)
		dc.LineTo(x+w-sk, y)
		dc.LineTo(x+w, y+h)
		dc.LineTo(x+sk, y+h)
		dc.ClosePath()
	case model.ShapeTrapezoid:
		sk := w * 0.18
		dc.MoveTo(x+sk, y)
		dc.LineTo(x+w-sk, y)
		dc.LineTo(x+w, y+h)
		dc.LineTo(x, y+h)
		dc.ClosePath()
	case model.ShapeTrapezoidAlt:
		sk := w * 0.18
		dc.MoveTo(x, y)
		dc.LineTo(x+w, y)
		dc.LineTo(x+w-sk, y+h)
		dc.LineTo(x+sk, y+h)
		dc.ClosePath()
	case model.ShapeCylinder:
		r.cylinderPath(dc, x, y, w, h)
	case model.ShapeSubroutine:
		dc.DrawRectangle(x, y, w, h)
		// side bars drawn separately after stroke in drawNode? keep simple:
	default: // ShapeRect
		dc.DrawRectangle(x, y, w, h)
	}
}

// cylinderPath approximates a database cylinder outline.
func (r *Renderer) cylinderPath(dc *gg.Context, x, y, w, h float64) {
	ry := math.Min(h*0.14, 12)
	// body as a rounded path: top ellipse arc + sides + bottom ellipse arc
	dc.NewSubPath()
	dc.MoveTo(x, y+ry)
	dc.DrawEllipticalArc(x+w/2, y+ry, w/2, ry, math.Pi, 2*math.Pi)
	dc.LineTo(x+w, y+h-ry)
	dc.DrawEllipticalArc(x+w/2, y+h-ry, w/2, ry, 0, math.Pi)
	dc.LineTo(x, y+ry)
	dc.ClosePath()
}

// roundRect adds a rounded rectangle path.
func (r *Renderer) roundRect(dc *gg.Context, x, y, w, h, rad float64) {
	if rad > w/2 {
		rad = w / 2
	}
	if rad > h/2 {
		rad = h / 2
	}
	dc.NewSubPath()
	dc.MoveTo(x+rad, y)
	dc.LineTo(x+w-rad, y)
	dc.DrawArc(x+w-rad, y+rad, rad, -math.Pi/2, 0)
	dc.LineTo(x+w, y+h-rad)
	dc.DrawArc(x+w-rad, y+h-rad, rad, 0, math.Pi/2)
	dc.LineTo(x+rad, y+h)
	dc.DrawArc(x+rad, y+h-rad, rad, math.Pi/2, math.Pi)
	dc.LineTo(x, y+rad)
	dc.DrawArc(x+rad, y+rad, rad, math.Pi, 3*math.Pi/2)
	dc.ClosePath()
}
