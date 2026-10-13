// Package seqrender rasterizes a laid-out sequence diagram to PNG using the
// pure-Go gg library, approximating mermaid's default sequence look: lavender
// actor boxes mirrored top and bottom, vertical lifelines, solid/dotted
// message arrows with labels, autonumber circles, yellow note boxes and block
// frames (loop/alt/opt/...).
package seqrender

import (
	"image/color"
	"math"

	"github.com/fogleman/gg"

	"github.com/megaproaktiv/gmaid/seqlayout"
	"github.com/megaproaktiv/gmaid/seqmodel"
	"github.com/megaproaktiv/gmaid/textmetrics"
)

// Theme holds the sequence-diagram palette.
type Theme struct {
	Background     color.Color
	ActorFill      color.Color
	ActorStroke    color.Color
	ActorText      color.Color
	Lifeline       color.Color
	Line           color.Color
	Text           color.Color
	NoteFill       color.Color
	NoteStroke     color.Color
	NoteText       color.Color
	NumberFill     color.Color
	NumberText     color.Color
	BlockStroke    color.Color
	BlockLabelFill color.Color
	BlockText      color.Color
}

// DefaultTheme returns a mermaid-like theme.
func DefaultTheme() Theme {
	return Theme{
		Background:     color.White,
		ActorFill:      rgb(0xEC, 0xEC, 0xFF),
		ActorStroke:    rgb(0x99, 0x70, 0xDB),
		ActorText:      rgb(0x1A, 0x1A, 0x1A),
		Lifeline:       rgb(0x99, 0x70, 0xDB),
		Line:           rgb(0x33, 0x33, 0x33),
		Text:           rgb(0x22, 0x22, 0x22),
		NoteFill:       rgb(0xFF, 0xF5, 0xAD),
		NoteStroke:     rgb(0xAA, 0xAA, 0x33),
		NoteText:       rgb(0x1A, 0x1A, 0x1A),
		NumberFill:     rgb(0x99, 0x70, 0xDB),
		NumberText:     color.White,
		BlockStroke:    rgb(0x99, 0x70, 0xDB),
		BlockLabelFill: rgb(0xEC, 0xEC, 0xFF),
		BlockText:      rgb(0x1A, 0x1A, 0x1A),
	}
}

// Renderer draws sequence diagrams.
type Renderer struct {
	theme     Theme
	cfg       seqlayout.Config
	scale     float64
	actorFace *textmetrics.Face
	msgFace   *textmetrics.Face
	numFace   *textmetrics.Face
}

// New returns a renderer.
func New(theme Theme, cfg seqlayout.Config, scale float64) *Renderer {
	if scale <= 0 {
		scale = 1
	}
	return &Renderer{
		theme:     theme,
		cfg:       cfg,
		scale:     scale,
		actorFace: textmetrics.New(cfg.FontSize, true),
		msgFace:   textmetrics.New(cfg.LabelFont, false),
		numFace:   textmetrics.New(11, true),
	}
}

// Render draws the diagram and writes a PNG to path.
func (r *Renderer) Render(d *seqlayout.Diagram, path string) error {
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

	// lifelines first (behind everything). Start at the bottom edge of the top
	// actor box so the box and its lifeline are visually connected (no gap).
	topBoxBottom := r.cfg.Margin + r.cfg.ActorH
	for _, a := range d.Actors {
		dc.SetColor(r.theme.Lifeline)
		dc.SetLineWidth(4)
		dc.SetDash()
		dc.DrawLine(a.CX, topBoxBottom, a.CX, d.LifelineBottom)
		dc.Stroke()
	}

	// block frames (behind messages, above lifelines)
	for _, b := range d.Blocks {
		r.drawBlock(dc, b)
	}

	// notes
	for _, nb := range d.Notes {
		r.drawNote(dc, nb)
	}

	// messages
	for _, m := range d.Messages {
		r.drawMessage(dc, m)
	}

	// actor boxes top and bottom (mirrored)
	for _, a := range d.Actors {
		r.drawActor(dc, a, r.cfg.Margin)     // top
		r.drawActor(dc, a, d.LifelineBottom) // bottom
	}

	return dc.SavePNG(path)
}

func (r *Renderer) drawActor(dc *gg.Context, a seqlayout.Actor, topY float64) {
	x := a.CX - a.W/2
	h := r.cfg.ActorH
	if a.Kind == seqmodel.KindActor {
		r.drawStickFigure(dc, a.CX, topY, a.Lines)
		return
	}
	dc.SetColor(r.theme.ActorFill)
	roundRect(dc, x, topY, a.W, h, 4)
	dc.FillPreserve()
	dc.SetColor(r.theme.ActorStroke)
	dc.SetLineWidth(1.3)
	dc.SetDash()
	dc.Stroke()

	dc.SetFontFace(r.actorFace.FontFace())
	dc.SetColor(r.theme.ActorText)
	total := float64(len(a.Lines)) * r.cfg.LineHeight
	cy := topY + h/2 - total/2 + r.cfg.LineHeight/2
	for i, ln := range a.Lines {
		dc.DrawStringAnchored(ln, a.CX, cy+float64(i)*r.cfg.LineHeight, 0.5, 0.5)
	}
}

func (r *Renderer) drawStickFigure(dc *gg.Context, cx, topY float64, lines []string) {
	// simple actor glyph
	h := r.cfg.ActorH
	headR := 6.0
	dc.SetColor(r.theme.ActorStroke)
	dc.SetLineWidth(1.3)
	dc.DrawCircle(cx, topY+headR, headR)
	dc.Stroke()
	dc.DrawLine(cx, topY+2*headR, cx, topY+h-10)
	dc.DrawLine(cx-10, topY+h-20, cx+10, topY+h-20) // arms
	dc.DrawLine(cx, topY+h-10, cx-8, topY+h)
	dc.DrawLine(cx, topY+h-10, cx+8, topY+h)
	dc.Stroke()
	dc.SetFontFace(r.actorFace.FontFace())
	dc.SetColor(r.theme.ActorText)
	for i, ln := range lines {
		dc.DrawStringAnchored(ln, cx, topY+h+8+float64(i)*r.cfg.LineHeight, 0.5, 0.5)
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
