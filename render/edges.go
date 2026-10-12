package render

import (
	"math"

	"github.com/fogleman/gg"

	"gmaid/layout"
	"gmaid/model"
)

// drawEdge renders an edge polyline with the appropriate line style and
// terminator (arrow/circle/cross/none).
func (r *Renderer) drawEdge(dc *gg.Context, e layout.EdgePath) {
	if len(e.Points) < 2 {
		return
	}
	lineW := 1.5
	dc.SetColor(r.theme.EdgeStroke)

	switch e.Edge.Line {
	case model.LineThick:
		lineW = 3.0
		dc.SetDash()
	case model.LineDotted:
		dc.SetDash(2.5, 3.0)
	default:
		dc.SetDash()
	}
	dc.SetLineWidth(lineW)

	// Shorten the final point to leave room for the arrowhead so the curve
	// does not poke through it.
	pts := append([]layout.Point(nil), e.Points...)
	prev := pts[len(pts)-2]
	trueEnd := pts[len(pts)-1]
	if e.Edge.Arrow == model.ArrowNormal {
		pts[len(pts)-1] = shorten(prev, trueEnd, 9)
	}

	// Build a smooth path through the routed points. With only two points this
	// is a straight line; with more it is a Catmull-Rom spline converted to
	// cubic Beziers, which bends gently through the dummy waypoints.
	endTangent := strokeSpline(dc, pts)
	dc.Stroke()
	dc.SetDash() // reset

	// terminator at the true endpoint, oriented along the curve's end tangent
	dir := endTangent
	if dir == (layout.Point{}) {
		dir = unit(sub(trueEnd, prev))
	}
	switch e.Edge.Arrow {
	case model.ArrowNormal:
		r.drawArrowHead(dc, trueEnd, dir)
	case model.ArrowCircle:
		dc.SetColor(r.theme.EdgeStroke)
		dc.DrawCircle(trueEnd.X-dir.X*4, trueEnd.Y-dir.Y*4, 4)
		dc.Fill()
	case model.ArrowCross:
		r.drawCross(dc, trueEnd)
	}
}

// strokeSpline lays a smooth path through pts onto the context (without
// stroking) and returns the unit tangent at the final point so the arrowhead
// can be oriented along the curve. For two points it emits a straight line.
func strokeSpline(dc *gg.Context, pts []layout.Point) layout.Point {
	n := len(pts)
	if n == 0 {
		return layout.Point{}
	}
	dc.MoveTo(pts[0].X, pts[0].Y)
	if n == 2 {
		dc.LineTo(pts[1].X, pts[1].Y)
		return unit(sub(pts[1], pts[0]))
	}

	// Catmull-Rom through all points -> cubic Bezier segments.
	// The tension-free (standard) Catmull-Rom control points for the segment
	// between p1 and p2 (with neighbors p0,p3) are:
	//   c1 = p1 + (p2 - p0)/6
	//   c2 = p2 - (p3 - p1)/6
	var endTan layout.Point
	for i := 0; i < n-1; i++ {
		p0 := pts[max0(i-1)]
		p1 := pts[i]
		p2 := pts[i+1]
		p3 := pts[min0(i+2, n-1)]

		c1 := layout.Point{X: p1.X + (p2.X-p0.X)/6, Y: p1.Y + (p2.Y-p0.Y)/6}
		c2 := layout.Point{X: p2.X - (p3.X-p1.X)/6, Y: p2.Y - (p3.Y-p1.Y)/6}
		dc.CubicTo(c1.X, c1.Y, c2.X, c2.Y, p2.X, p2.Y)

		if i == n-2 {
			// tangent at end of a cubic Bezier is p2 - c2
			endTan = unit(sub(p2, c2))
		}
	}
	return endTan
}

func max0(a int) int {
	if a < 0 {
		return 0
	}
	return a
}
func min0(a, b int) int {
	if a > b {
		return b
	}
	return a
}

// drawArrowHead draws a filled triangle at p pointing along dir.
func (r *Renderer) drawArrowHead(dc *gg.Context, p layout.Point, dir layout.Point) {
	const size = 10
	const half = 4.0
	// base point behind the tip
	base := layout.Point{X: p.X - dir.X*size, Y: p.Y - dir.Y*size}
	// perpendicular
	nx, ny := -dir.Y, dir.X
	a := layout.Point{X: base.X + nx*half, Y: base.Y + ny*half}
	b := layout.Point{X: base.X - nx*half, Y: base.Y - ny*half}
	dc.SetColor(r.theme.EdgeStroke)
	dc.MoveTo(p.X, p.Y)
	dc.LineTo(a.X, a.Y)
	dc.LineTo(b.X, b.Y)
	dc.ClosePath()
	dc.Fill()
}

func (r *Renderer) drawCross(dc *gg.Context, p layout.Point) {
	const s = 5
	dc.SetColor(r.theme.EdgeStroke)
	dc.SetLineWidth(1.6)
	dc.DrawLine(p.X-s, p.Y-s, p.X+s, p.Y+s)
	dc.DrawLine(p.X-s, p.Y+s, p.X+s, p.Y-s)
	dc.Stroke()
}

// drawEdgeLabel draws the edge label with a solid background chip for legibility.
func (r *Renderer) drawEdgeLabel(dc *gg.Context, e layout.EdgePath) {
	dc.SetFontFace(r.face.FontFace())
	tw, th := r.face.Measure(e.Label)
	pad := 3.0
	x := e.LabelPos.X
	y := e.LabelPos.Y
	dc.SetColor(r.theme.LabelBG)
	dc.DrawRectangle(x-tw/2-pad, y-th/2-pad, tw+2*pad, th+2*pad)
	dc.Fill()
	dc.SetColor(r.theme.EdgeText)
	dc.DrawStringAnchored(e.Label, x, y, 0.5, 0.5)
}

// --- small vector helpers ---

func sub(a, b layout.Point) layout.Point { return layout.Point{X: a.X - b.X, Y: a.Y - b.Y} }

func unit(v layout.Point) layout.Point {
	l := math.Hypot(v.X, v.Y)
	if l == 0 {
		return layout.Point{X: 0, Y: 0}
	}
	return layout.Point{X: v.X / l, Y: v.Y / l}
}

// shorten returns a point moved d units back from b toward a.
func shorten(a, b layout.Point, d float64) layout.Point {
	u := unit(sub(b, a))
	return layout.Point{X: b.X - u.X*d, Y: b.Y - u.Y*d}
}
