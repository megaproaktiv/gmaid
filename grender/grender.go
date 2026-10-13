// Package grender rasterizes a laid-out gantt chart to PNG using the pure-Go gg
// library, approximating mermaid's default gantt look: centered title,
// alternating section bands, task bars colored by tag (active=purple, crit=red,
// done=grey, normal=light blue), milestone diamonds, a bottom date axis with
// vertical gridlines, and a red today marker.
package grender

import (
	"image/color"
	"math"

	"github.com/fogleman/gg"

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
	}
}

// Renderer draws gantt charts.
type Renderer struct {
	theme    Theme
	cfg      glayout.Config
	scale    float64
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

	// section names (left)
	dc.SetFontFace(r.sectionF.FontFace())
	for _, s := range d.Sections {
		if s.Name == "" {
			continue
		}
		dc.SetColor(r.theme.SectionText)
		dc.DrawStringAnchored(s.Name, r.cfg.Margin+6, s.LabelY, 0, 0.5)
	}

	// bars + milestones
	for _, b := range d.Bars {
		r.drawBar(dc, b)
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

func (r *Renderer) drawBar(dc *gg.Context, b glayout.Bar) {
	if b.Milestone {
		r.drawMilestone(dc, b)
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
	roundRect(dc, b.X, b.Y, b.W, b.H, 3)
	dc.SetColor(fill)
	dc.FillPreserve()
	dc.SetColor(r.theme.BarStroke)
	dc.SetLineWidth(1.2)
	dc.Stroke()

	dc.SetFontFace(r.face.FontFace())
	if b.LabelInside {
		dc.SetColor(r.theme.BarText)
		dc.DrawStringAnchored(b.Label, b.LabelX, b.Y+b.H/2, 0.5, 0.5)
	} else {
		dc.SetColor(r.theme.OutsideText)
		dc.DrawStringAnchored(b.Label, b.LabelX, b.Y+b.H/2, 0, 0.5)
	}
}

func (r *Renderer) drawMilestone(dc *gg.Context, b glayout.Bar) {
	fill := r.theme.BarActive
	switch b.Task.Tag {
	case gmodel.TagDone:
		fill = r.theme.BarDone
	case gmodel.TagCrit:
		fill = r.theme.BarCrit
	}
	s := b.H / 2
	dc.MoveTo(b.MX, b.MY-s)
	dc.LineTo(b.MX+s, b.MY)
	dc.LineTo(b.MX, b.MY+s)
	dc.LineTo(b.MX-s, b.MY)
	dc.ClosePath()
	dc.SetColor(fill)
	dc.FillPreserve()
	dc.SetColor(r.theme.BarStroke)
	dc.SetLineWidth(1.2)
	dc.Stroke()

	dc.SetFontFace(r.italF.FontFace())
	dc.SetColor(r.theme.OutsideText)
	dc.DrawStringAnchored(b.Label, b.LabelX, b.MY, 0, 0.5)
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
