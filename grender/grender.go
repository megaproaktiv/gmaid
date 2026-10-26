// Package grender rasterizes a laid-out gantt chart to PNG using the pure-Go gg
// library, approximating mermaid's default gantt look: centered title,
// alternating section bands, task bars colored by tag (active=purple, crit=red,
// done=grey, normal=light blue), milestone diamonds, a bottom date axis with
// vertical gridlines, and a red today marker.
package grender

import (
	"image/color"
	"math"
	"strings"

	"github.com/fogleman/gg"

	"github.com/megaproaktiv/gmaid/gcss"
	"github.com/megaproaktiv/gmaid/glayout"
	"github.com/megaproaktiv/gmaid/gmodel"
	"github.com/megaproaktiv/gmaid/textmetrics"
)

// Theme holds the gantt palette.
type Theme struct {
	Background  color.Color
	Title       color.Color
	Bands       []color.Color // section band colors, cycled by section index
	BarNormal   color.Color
	BarActive   color.Color
	BarDone     color.Color
	BarCrit     color.Color
	BarStroke   color.Color
	BarText     color.Color
	OutsideText color.Color
	Grid        color.Color
	AxisText    color.Color
	Today       color.Color
	SectionText color.Color
	Link        color.Color
	Summary     color.Color
}

// DefaultTheme returns a mermaid-like gantt theme.
func DefaultTheme() Theme {
	return Theme{
		Background: color.White,
		Title:      rgb(0x33, 0x33, 0x33),
		Bands: []color.Color{
			rgb(0xE6, 0xE6, 0xFA), // light lavender
			rgb(0xF7, 0xF7, 0xFF), // near white
			rgb(0xFC, 0xFC, 0xE3), // light yellow
		},
		BarNormal:   rgb(0x8A, 0x90, 0xDD),
		BarActive:   rgb(0x6A, 0x6A, 0xD6),
		BarDone:     rgb(0xBF, 0xC7, 0xD5),
		BarCrit:     rgb(0xFF, 0x37, 0x37),
		BarStroke:   rgb(0x53, 0x4F, 0xBC),
		BarText:     color.White,
		OutsideText: rgb(0x1A, 0x1A, 0x1A),
		Grid:        rgb(0xD0, 0xD0, 0xD0),
		AxisText:    rgb(0x33, 0x33, 0x33),
		Today:       rgb(0xFF, 0x00, 0x00),
		SectionText: rgb(0x33, 0x33, 0x33),
		Link:        rgb(0x33, 0x33, 0x33),
		Summary:     rgb(0x1A, 0x1A, 0x1A),
	}
}

// Renderer draws gantt charts.
type Renderer struct {
	theme    Theme
	cfg      glayout.Config
	scale    float64
	style    *gcss.Sheet // optional external stylesheet (gmaid -C), may be nil
	face     *textmetrics.Face
	titleF   *textmetrics.Face
	sectionF *textmetrics.Face
	italF    *textmetrics.Face
}

// New returns a renderer.
func New(theme Theme, cfg glayout.Config, scale float64) *Renderer {
	if scale <= 0 {
		scale = 1
	}
	return &Renderer{
		theme:    theme,
		cfg:      cfg,
		scale:    scale,
		face:     textmetrics.New(cfg.FontSize, false),
		titleF:   textmetrics.New(20, true),
		sectionF: textmetrics.New(cfg.SectionFont, false),
		italF:    textmetrics.New(cfg.FontSize, false),
	}
}

// SetStyle attaches an external stylesheet used to override per-task bar
// colors (fill/stroke) by task id. Pass nil to clear.
func (r *Renderer) SetStyle(s *gcss.Sheet) { r.style = s }

// Render draws the chart and writes a PNG to path.
func (r *Renderer) Render(d *glayout.Diagram, path string) error {
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

	// section bands
	for _, s := range d.Sections {
		band := r.theme.Bands[s.Index%len(r.theme.Bands)]
		dc.SetColor(band)
		dc.DrawRectangle(s.X, s.Y, s.W, s.H)
		dc.Fill()
	}

	// vertical gridlines (behind bars)
	for _, t := range d.Ticks {
		dc.SetColor(r.theme.Grid)
		dc.SetLineWidth(1)
		dc.DrawLine(t.X, d.ChartTop, t.X, d.ChartBottom)
		dc.Stroke()
	}

	// section names (left), supports multi-line via \n
	dc.SetFontFace(r.sectionF.FontFace())
	for _, s := range d.Sections {
		if s.Name == "" {
			continue
		}
		dc.SetColor(r.theme.SectionText)
		lines := s.LabelLines
		if len(lines) == 0 {
			lines = textmetrics.SplitLines(s.Name)
		}
		lh := s.LineH
		if lh <= 0 {
			lh = r.cfg.SectionFont + 3
		}
		startY := s.LabelY - float64(len(lines)-1)*lh/2
		for i, ln := range lines {
			r.drawLabel(dc, r.sectionF, ln, r.cfg.Margin+6, startY+float64(i)*lh, 0)
		}
	}

	// bars + milestones (shapes only; labels drawn last so text stays on top)
	for _, b := range d.Bars {
		r.drawBar(dc, b)
	}

	// dependency arrows (on top of bars, below labels)
	for _, lk := range d.Links {
		r.drawLink(dc, lk)
	}

	// labels on top of ALL blocks and arrows
	for _, b := range d.Bars {
		r.drawBarLabel(dc, b)
	}

	// axis
	r.drawAxis(dc, d)

	// today marker (on top)
	if d.TodayX >= 0 {
		dc.SetColor(r.theme.Today)
		dc.SetLineWidth(2)
		dc.DrawLine(d.TodayX, d.ChartTop, d.TodayX, d.ChartBottom)
		dc.Stroke()
	}

	// title
	if d.Title != "" {
		dc.SetFontFace(r.titleF.FontFace())
		dc.SetColor(r.theme.Title)
		dc.DrawStringAnchored(d.Title, d.Width/2, r.cfg.Margin+20, 0.5, 0.5)
	}

	return dc.SavePNG(path)
}

// combining macron / overline runes that the built-in Go font cannot render.
const (
	combMacron   = '\u0304'
	combOverline = '\u0305'
)

// hasOverline reports whether s contains a combining macron/overline.
func hasOverline(s string) bool {
	return strings.ContainsRune(s, combMacron) || strings.ContainsRune(s, combOverline)
}

// drawLabel draws a single line of text anchored at (x,y) with horizontal
// anchor ax (0 = left, 0.5 = center), emulating combining macron/overline
// (U+0304/U+0305) by stripping the mark and drawing a short bar above the
// preceding base character (the Go built-in font has no glyph for these marks,
// so they would otherwise render as tofu boxes).
func (r *Renderer) drawLabel(dc *gg.Context, face *textmetrics.Face, s string, x, y, ax float64) {
	if !hasOverline(s) {
		dc.DrawStringAnchored(s, x, y, ax, 0.5)
		return
	}
	// Build the plain string (marks removed) and record, for each base rune
	// that carries a mark, the [start,end) rune span to overline.
	var plain strings.Builder
	type span struct{ a, b int } // byte offsets into plain
	var spans []span
	lastStart := -1
	for _, rn := range s {
		if rn == combMacron || rn == combOverline {
			if lastStart >= 0 {
				spans = append(spans, span{lastStart, plain.Len()})
			}
			continue
		}
		lastStart = plain.Len()
		plain.WriteRune(rn)
	}
	text := plain.String()
	totalW, _ := face.Measure(text)
	// Left edge of the text given the anchor.
	left := x - ax*totalW
	dc.DrawStringAnchored(text, left, y, 0, 0.5)
	// Draw an overline bar above each marked base glyph.
	for _, sp := range spans {
		preW, _ := face.Measure(text[:sp.a])
		segW, _ := face.Measure(text[sp.a:sp.b])
		x0 := left + preW
		x1 := x0 + segW
		barY := y - r.cfg.FontSize*0.62
		dc.SetLineWidth(1)
		dc.DrawLine(x0, barY, x1, barY)
		dc.Stroke()
	}
}

func (r *Renderer) drawBar(dc *gg.Context, b glayout.Bar) {
	if b.Milestone {
		r.drawMilestone(dc, b)
		return
	}
	if b.Task.Summary {
		r.drawSummary(dc, b)
		return
	}
	fill := r.theme.BarNormal
	switch b.Task.Tag {
	case gmodel.TagActive:
		fill = r.theme.BarActive
	case gmodel.TagDone:
		fill = r.theme.BarDone
	case gmodel.TagCrit:
		fill = r.theme.BarCrit
	}
	stroke := r.theme.BarStroke
	// External stylesheet overrides fill/stroke by task id.
	if r.style != nil && b.Task.ID != "" {
		if f, s, hf, hs, ok := r.style.Match(b.Task.ID); ok {
			if hf {
				fill = f
			}
			if hs {
				stroke = s
			}
		}
	}
	roundRect(dc, b.X, b.Y, b.W, b.H, 3)
	dc.SetColor(fill)
	dc.FillPreserve()
	dc.SetColor(stroke)
	dc.SetLineWidth(1.2)
	dc.Stroke()
}

// drawBarLabel draws the text for a bar/milestone/summary. It is called in a
// final pass AFTER all bars and dependency arrows so labels always stay on top
// of every block and never get overdrawn.
func (r *Renderer) drawBarLabel(dc *gg.Context, b glayout.Bar) {
	if b.Milestone {
		dc.SetFontFace(r.italF.FontFace())
		dc.SetColor(r.theme.OutsideText)
		r.drawLabel(dc, r.italF, b.Label, b.LabelX, b.MY, 0)
		return
	}
	if b.Task.Summary {
		dc.SetFontFace(r.face.FontFace())
		dc.SetColor(r.theme.OutsideText)
		r.drawLabel(dc, r.face, b.Label, b.X+b.W+6, b.Y+b.H/2, 0)
		return
	}
	dc.SetFontFace(r.face.FontFace())
	lines := textmetrics.SplitLines(b.Label)
	lh := r.cfg.FontSize + 3
	cy := b.Y + b.H/2
	startY := cy - float64(len(lines)-1)*lh/2
	if b.LabelInside {
		dc.SetColor(r.theme.BarText)
		for i, ln := range lines {
			r.drawLabel(dc, r.face, ln, b.LabelX, startY+float64(i)*lh, 0.5)
		}
	} else {
		dc.SetColor(r.theme.OutsideText)
		for i, ln := range lines {
			r.drawLabel(dc, r.face, ln, b.LabelX, startY+float64(i)*lh, 0)
		}
	}
}

// drawSummary draws an MS-Project-style summary (rollup) bar: a slim solid
// black bar spanning the range, with a downward-pointing filled triangle at
// each end (the characteristic "bracket" look).
func (r *Renderer) drawSummary(dc *gg.Context, b glayout.Bar) {
	col := r.theme.Summary
	if r.style != nil && b.Task.ID != "" {
		if f, _, hf, _, ok := r.style.Match(b.Task.ID); ok && hf {
			col = f
		}
	}
	barTop := b.Y + b.H*0.18
	barH := b.H * 0.34
	if barH < 3 {
		barH = 3
	}
	// main spine
	dc.SetColor(col)
	dc.DrawRectangle(b.X, barTop, b.W, barH)
	dc.Fill()

	// downward triangles at both ends
	tw := b.H * 0.42 // triangle half-width-ish
	th := b.H * 0.52 // triangle height (points down)
	topY := barTop + barH
	// left end cap (apex points down, base aligned with left edge)
	dc.MoveTo(b.X, topY)
	dc.LineTo(b.X+tw, topY)
	dc.LineTo(b.X, topY+th)
	dc.ClosePath()
	dc.SetColor(col)
	dc.Fill()
	// right end cap
	dc.MoveTo(b.X+b.W, topY)
	dc.LineTo(b.X+b.W-tw, topY)
	dc.LineTo(b.X+b.W, topY+th)
	dc.ClosePath()
	dc.SetColor(col)
	dc.Fill()
	// label is drawn later by drawBarLabel (kept on top of all blocks)
}

func (r *Renderer) drawMilestone(dc *gg.Context, b glayout.Bar) {
	fill := r.theme.BarActive
	switch b.Task.Tag {
	case gmodel.TagDone:
		fill = r.theme.BarDone
	case gmodel.TagCrit:
		fill = r.theme.BarCrit
	}
	stroke := r.theme.BarStroke
	if r.style != nil && b.Task.ID != "" {
		if f, s, hf, hs, ok := r.style.Match(b.Task.ID); ok {
			if hf {
				fill = f
			}
			if hs {
				stroke = s
			}
		}
	}
	s := b.H / 2
	dc.MoveTo(b.MX, b.MY-s)
	dc.LineTo(b.MX+s, b.MY)
	dc.LineTo(b.MX, b.MY+s)
	dc.LineTo(b.MX-s, b.MY)
	dc.ClosePath()
	dc.SetColor(fill)
	dc.FillPreserve()
	dc.SetColor(stroke)
	dc.SetLineWidth(1.2)
	dc.Stroke()
	// label is drawn later by drawBarLabel (kept on top of all blocks)
}

func (r *Renderer) drawLink(dc *gg.Context, lk glayout.LinkPath) {
	if len(lk.Pts) < 2 {
		return
	}
	dc.SetColor(r.theme.Link)
	dc.SetLineWidth(1.3)
	// Collapse consecutive duplicate points to avoid zero-length segments.
	pts := lk.Pts[:1]
	for _, p := range lk.Pts[1:] {
		last := pts[len(pts)-1]
		if p.X != last.X || p.Y != last.Y {
			pts = append(pts, p)
		}
	}
	if len(pts) < 2 {
		return
	}
	dc.MoveTo(pts[0].X, pts[0].Y)
	for _, p := range pts[1:] {
		dc.LineTo(p.X, p.Y)
	}
	dc.Stroke()

	// Arrowhead at the final segment.
	a := pts[len(pts)-2]
	b := pts[len(pts)-1]
	dx, dy := b.X-a.X, b.Y-a.Y
	l := math.Hypot(dx, dy)
	if l == 0 {
		return
	}
	ux, uy := dx/l, dy/l
	const ah = 7.0 // arrowhead length
	const aw = 3.5 // half width
	// base point behind the tip
	bx, by := b.X-ux*ah, b.Y-uy*ah
	// perpendicular
	px, py := -uy, ux
	dc.MoveTo(b.X, b.Y)
	dc.LineTo(bx+px*aw, by+py*aw)
	dc.LineTo(bx-px*aw, by-py*aw)
	dc.ClosePath()
	dc.SetColor(r.theme.Link)
	dc.Fill()
}

func (r *Renderer) drawAxis(dc *gg.Context, d *glayout.Diagram) {
	dc.SetFontFace(r.face.FontFace())
	dc.SetColor(r.theme.AxisText)
	for _, t := range d.Ticks {
		dc.DrawStringAnchored(t.Label, t.X, d.ChartBottom+14, 0.5, 0.5)
	}
}

func rgb(r, g, b uint8) color.Color { return color.RGBA{r, g, b, 255} }

// roundRect adds a rounded-rectangle path.
func roundRect(dc *gg.Context, x, y, w, h, rad float64) {
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
