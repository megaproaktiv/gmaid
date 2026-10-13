// Package gitrender rasterizes a laid-out gitGraph to PNG using the pure-Go gg
// library, approximating mermaid's default gitGraph look: horizontal branch
// lanes with colored rounded label boxes on the left, thick colored connectors
// with rounded elbows between lanes, filled commit circles (ringed for merges,
// squares for highlights, crossed for reverses), tags above commits and rotated
// commit ids below.
package gitrender

import (
	"image/color"
	"math"

	"github.com/fogleman/gg"

	"github.com/megaproaktiv/gmaid/gitlayout"
	"github.com/megaproaktiv/gmaid/gitmodel"
	"github.com/megaproaktiv/gmaid/textmetrics"
)

// Theme holds the gitGraph palette.
type Theme struct {
	Background color.Color
	Branches   []color.Color // cycled by branch creation index
	LabelText  color.Color   // text inside branch label boxes (white)
	CommitID   color.Color   // rotated commit id text
	TagText    color.Color
	TagFill    color.Color
	TagStroke  color.Color
	MergeInner color.Color // inner fill of a merge (ringed) circle
}

// DefaultTheme returns a mermaid-like gitGraph theme. Branch colors were
// sampled from mmdc reference output.
func DefaultTheme() Theme {
	return Theme{
		Background: color.White,
		Branches: []color.Color{
			rgb(0x00, 0x00, 0xE0), // 0 main   blue
			rgb(0xD0, 0xD0, 0x00), // 1 yellow
			rgb(0x90, 0xE0, 0x00), // 2 green
			rgb(0x00, 0x70, 0xE0), // 3 bright blue
			rgb(0xE0, 0x60, 0x00), // 4 orange
			rgb(0x9F, 0x00, 0xE0), // 5 purple
			rgb(0x00, 0xC0, 0xB0), // 6 teal
			rgb(0xE0, 0x00, 0x60), // 7 pink
		},
		LabelText:  color.White,
		CommitID:   rgb(0x33, 0x33, 0x33),
		TagText:    rgb(0x33, 0x33, 0x33),
		TagFill:    rgb(0xFF, 0xFF, 0xCC),
		TagStroke:  rgb(0xAA, 0xAA, 0xAA),
		MergeInner: color.White,
	}
}

// Renderer draws gitGraph diagrams.
type Renderer struct {
	theme   Theme
	cfg     gitlayout.Config
	scale   float64
	labelF  *textmetrics.Face
	idF     *textmetrics.Face
	tagF    *textmetrics.Face
}

// New returns a renderer.
func New(theme Theme, cfg gitlayout.Config, scale float64) *Renderer {
	if scale <= 0 {
		scale = 1
	}
	return &Renderer{
		theme:  theme,
		cfg:    cfg,
		scale:  scale,
		labelF: textmetrics.New(cfg.LabelFont, true),
		idF:    textmetrics.New(cfg.FontSize, false),
		tagF:   textmetrics.New(cfg.TagFont, false),
	}
}

func (r *Renderer) branchColor(i int) color.Color {
	p := r.theme.Branches
	return p[((i%len(p))+len(p))%len(p)]
}

// Render draws the diagram and writes a PNG to path.
func (r *Renderer) Render(d *gitlayout.Diagram, path string) error {
	w := int(d.Width*r.scale + 0.5)
	h := int(d.Height*r.scale + 0.5)
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	dc := gg.NewContext(w, h)
	dc.Scale(r.scale, r.scale)
	dc.SetColor(r.theme.Background)
	dc.Clear()

	// 1. connectors (behind commits)
	for _, c := range d.Connectors {
		r.drawConnector(dc, c)
	}

	// 2. commit circles
	for _, dot := range d.Dots {
		r.drawDot(dc, dot)
	}

	// 3. tags (above commits)
	dc.SetFontFace(r.tagF.FontFace())
	for _, dot := range d.Dots {
		if dot.Tag != "" {
			r.drawTag(dc, dot)
		}
	}

	// 4. rotated commit ids (below commits)
	for _, dot := range d.Dots {
		r.drawCommitID(dc, dot)
	}

	// 5. branch label boxes (left) on top
	for _, l := range d.Labels {
		r.drawLabel(dc, l)
	}

	return dc.SavePNG(path)
}

// drawConnector draws a thick colored path from (X0,Y0) to (X1,Y1). When the
// two endpoints are on different lanes it uses a rounded elbow like mermaid.
func (r *Renderer) drawConnector(dc *gg.Context, c gitlayout.Connector) {
	col := r.branchColor(c.ColorIndex)
	dc.SetColor(col)
	dc.SetLineWidth(r.cfg.Radius * 0.9)
	dc.SetLineCapRound()
	dc.SetLineJoinRound()

	if math.Abs(c.Y0-c.Y1) < 0.5 {
		// same lane: straight horizontal
		dc.MoveTo(c.X0, c.Y0)
		dc.LineTo(c.X1, c.Y1)
		dc.Stroke()
		return
	}

	// Different lanes: rounded elbow. mermaid turns near the child's x: run
	// horizontally from the source, then curve vertically into the child lane.
	rad := math.Min(r.cfg.LaneGap/2, math.Abs(c.X1-c.X0)/2)
	if rad < 1 {
		rad = 1
	}
	dir := 1.0
	if c.Y1 < c.Y0 {
		dir = -1.0
	}
	// corner point just before the child's column
	cornerX := c.X1 - rad
	if cornerX < c.X0 {
		cornerX = c.X0
	}
	dc.MoveTo(c.X0, c.Y0)
	dc.LineTo(cornerX, c.Y0)
	// quadratic curve around the corner into the vertical, then down to child
	dc.QuadraticTo(c.X1, c.Y0, c.X1, c.Y0+dir*rad)
	dc.LineTo(c.X1, c.Y1)
	dc.Stroke()
}

func (r *Renderer) drawDot(dc *gg.Context, dot gitlayout.CommitDot) {
	col := r.branchColor(dot.Branch)
	rad := r.cfg.Radius
	switch dot.Kind {
	case gitmodel.CommitMerge:
		// hollow ring: colored outer, white inner, colored small core
		dc.SetColor(col)
		dc.DrawCircle(dot.X, dot.Y, rad)
		dc.Fill()
		dc.SetColor(r.theme.MergeInner)
		dc.DrawCircle(dot.X, dot.Y, rad*0.6)
		dc.Fill()
		dc.SetColor(col)
		dc.DrawCircle(dot.X, dot.Y, rad*0.28)
		dc.Fill()
	case gitmodel.CommitHighlight:
		s := rad * 1.3
		dc.SetColor(col)
		dc.DrawRectangle(dot.X-s, dot.Y-s, 2*s, 2*s)
		dc.Fill()
	case gitmodel.CommitReverse:
		dc.SetColor(col)
		dc.DrawCircle(dot.X, dot.Y, rad)
		dc.Fill()
		dc.SetColor(r.theme.Background)
		dc.SetLineWidth(2)
		dc.DrawLine(dot.X-rad*0.6, dot.Y-rad*0.6, dot.X+rad*0.6, dot.Y+rad*0.6)
		dc.DrawLine(dot.X-rad*0.6, dot.Y+rad*0.6, dot.X+rad*0.6, dot.Y-rad*0.6)
		dc.Stroke()
	default: // normal + cherry-pick
		dc.SetColor(col)
		dc.DrawCircle(dot.X, dot.Y, rad)
		dc.Fill()
	}
}

func (r *Renderer) drawTag(dc *gg.Context, dot gitlayout.CommitDot) {
	tw, th := r.tagF.Measure(dot.Tag)
	padX, padY := 6.0, 3.0
	boxW := tw + 2*padX
	boxH := th + 2*padY
	bx := dot.X - boxW/2
	by := dot.TagY - boxH/2
	// small connector bullet from tag down to commit
	dc.SetColor(r.theme.TagStroke)
	dc.SetLineWidth(1)
	dc.DrawLine(dot.X, by+boxH, dot.X, dot.Y-r.cfg.Radius)
	dc.Stroke()
	// tag box
	dc.SetColor(r.theme.TagFill)
	dc.DrawRoundedRectangle(bx, by, boxW, boxH, 3)
	dc.FillPreserve()
	dc.SetColor(r.theme.TagStroke)
	dc.SetLineWidth(1)
	dc.Stroke()
	// bullet dot
	dc.SetColor(r.theme.TagStroke)
	dc.DrawCircle(bx+3, by+boxH/2, 1.6)
	dc.Fill()
	// text
	dc.SetColor(r.theme.TagText)
	dc.DrawStringAnchored(dot.Tag, dot.X+2, by+boxH/2, 0.5, 0.5)
}

func (r *Renderer) drawCommitID(dc *gg.Context, dot gitlayout.CommitDot) {
	if dot.ID == "" || !dot.ShowID {
		return
	}
	dc.SetFontFace(r.idF.FontFace())
	dc.SetColor(r.theme.CommitID)
	// rotated ~45deg below the commit, like mermaid
	dc.Push()
	dc.RotateAbout(gg.Radians(45), dot.X, dot.Y+r.cfg.Radius+6)
	dc.DrawStringAnchored(dot.ID, dot.X, dot.Y+r.cfg.Radius+6, 0, 1)
	dc.Pop()
}

func (r *Renderer) drawLabel(dc *gg.Context, l gitlayout.LaneLabel) {
	col := r.branchColor(l.Index)
	dc.SetColor(col)
	dc.DrawRoundedRectangle(l.X, l.Y, l.W, l.H, 5)
	dc.Fill()
	dc.SetFontFace(r.labelF.FontFace())
	dc.SetColor(r.theme.LabelText)
	dc.DrawStringAnchored(l.Name, l.X+l.W/2, l.CY, 0.5, 0.5)
}

func rgb(r, g, b uint8) color.Color { return color.RGBA{r, g, b, 255} }
