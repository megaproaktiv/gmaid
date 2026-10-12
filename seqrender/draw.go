package seqrender

import (
	"github.com/fogleman/gg"

	"gmaid/seqlayout"
	"gmaid/seqmodel"
	"gmaid/textmetrics"
)

// drawMessage renders a message arrow (straight or self-loop), its label and
// optional autonumber circle.
func (r *Renderer) drawMessage(dc *gg.Context, m seqlayout.Message) {
	dc.SetColor(r.theme.Line)
	dotted := m.Ev.Arrow.Dotted
	if dotted {
		dc.SetDash(3, 3)
	} else {
		dc.SetDash()
	}
	dc.SetLineWidth(2.8)

	if m.Self {
		r.drawSelfMessage(dc, m)
	} else {
		y := m.Y
		// label centered above the line
		r.drawMsgLabel(dc, m, (m.FromX+m.ToX)/2, m.LabelY)
		dc.SetColor(r.theme.Line)
		dc.DrawLine(m.FromX, y, m.ToX, y)
		dc.Stroke()
		dc.SetDash()
		r.drawHead(dc, m.ToX, y, m.FromX < m.ToX, m.Ev.Arrow.Head)
		if m.Ev.Arrow.Head == seqmodel.HeadBi {
			r.drawHead(dc, m.FromX, y, m.FromX > m.ToX, seqmodel.HeadArrow)
		}
	}
	r.drawNumber(dc, m)
}

func (r *Renderer) drawSelfMessage(dc *gg.Context, m seqlayout.Message) {
	x := m.FromX
	top := m.LabelY + 2
	bot := m.Y
	w := r.cfg.SelfMsgGap
	// label to the right of the loop
	r.drawMsgLabelLeft(dc, m, x+w+6, m.LabelY)
	dc.SetColor(r.theme.Line)

	// Rounded self-message loop: out to the right, curve down, back to the
	// lifeline, using quadratic corners for a smooth bend.
	rad := (bot - top) / 2
	if rad > w {
		rad = w
	}
	dc.NewSubPath()
	dc.MoveTo(x, top)
	dc.LineTo(x+w-rad, top)
	dc.QuadraticTo(x+w, top, x+w, top+rad) // top-right rounded corner
	dc.LineTo(x+w, bot-rad)
	dc.QuadraticTo(x+w, bot, x+w-rad, bot) // bottom-right rounded corner
	dc.LineTo(x, bot)
	dc.Stroke()
	dc.SetDash()
	// Self lines always get an arrowhead pointing back into the lifeline.
	head := m.Ev.Arrow.Head
	if head == seqmodel.HeadNone {
		head = seqmodel.HeadArrow
	}
	r.drawHead(dc, x, bot, false, head)
}

// drawHead draws the terminator at (x,y). rightward indicates arrow direction.
func (r *Renderer) drawHead(dc *gg.Context, x, y float64, rightward bool, head seqmodel.HeadType) {
	dir := 1.0
	if !rightward {
		dir = -1.0
	}
	dc.SetColor(r.theme.Line)
	switch head {
	case seqmodel.HeadArrow, seqmodel.HeadBi:
		const size = 9
		const half = 4.0
		tipX := x
		baseX := x - dir*size
		dc.MoveTo(tipX, y)
		dc.LineTo(baseX, y-half)
		dc.LineTo(baseX, y+half)
		dc.ClosePath()
		dc.Fill()
	case seqmodel.HeadOpen:
		const size = 8
		const half = 4.0
		dc.SetLineWidth(1.4)
		dc.DrawLine(x, y, x-dir*size, y-half)
		dc.DrawLine(x, y, x-dir*size, y+half)
		dc.Stroke()
	case seqmodel.HeadCross:
		const s = 4.0
		dc.SetLineWidth(1.6)
		dc.DrawLine(x-s, y-s, x+s, y+s)
		dc.DrawLine(x-s, y+s, x+s, y-s)
		dc.Stroke()
	}
}

func (r *Renderer) drawMsgLabel(dc *gg.Context, m seqlayout.Message, cx, topY float64) {
	dc.SetFontFace(r.msgFace.FontFace())
	dc.SetColor(r.theme.Text)
	for i, ln := range m.Lines {
		dc.DrawStringAnchored(ln, cx, topY+float64(i)*r.cfg.LineHeight+r.cfg.LineHeight/2, 0.5, 0.5)
	}
}

func (r *Renderer) drawMsgLabelLeft(dc *gg.Context, m seqlayout.Message, x, topY float64) {
	dc.SetFontFace(r.msgFace.FontFace())
	dc.SetColor(r.theme.Text)
	for i, ln := range m.Lines {
		dc.DrawStringAnchored(ln, x, topY+float64(i)*r.cfg.LineHeight+r.cfg.LineHeight/2, 0, 0.5)
	}
}

// drawNumber draws the autonumber circle at the message source. For a self
// message the circle sits at the loop's start (top), so the line visually
// begins in the number and then bends down to the bottom.
func (r *Renderer) drawNumber(dc *gg.Context, m seqlayout.Message) {
	if m.Number == 0 {
		return
	}
	cx := m.FromX
	cy := m.Y
	if m.Self {
		cy = m.LabelY + 2
	}
	const rad = 9.0
	dc.SetColor(r.theme.NumberFill)
	dc.DrawCircle(cx, cy, rad)
	dc.Fill()
	dc.SetFontFace(r.numFace.FontFace())
	dc.SetColor(r.theme.NumberText)
	dc.DrawStringAnchored(itoa(m.Number), cx, cy, 0.5, 0.5)
}

// drawNote renders a note box.
func (r *Renderer) drawNote(dc *gg.Context, nb seqlayout.NoteBox) {
	dc.SetColor(r.theme.NoteFill)
	dc.DrawRectangle(nb.X, nb.Y, nb.W, nb.H)
	dc.FillPreserve()
	dc.SetColor(r.theme.NoteStroke)
	dc.SetLineWidth(1)
	dc.SetDash()
	dc.Stroke()
	dc.SetFontFace(r.msgFace.FontFace())
	dc.SetColor(r.theme.NoteText)
	total := float64(len(nb.Lines)) * r.cfg.LineHeight
	cy := nb.Y + nb.H/2 - total/2 + r.cfg.LineHeight/2
	for i, ln := range nb.Lines {
		dc.DrawStringAnchored(ln, nb.X+nb.W/2, cy+float64(i)*r.cfg.LineHeight, 0.5, 0.5)
	}
}

// drawBlock renders a loop/alt/opt/par frame with its label tab and dividers.
func (r *Renderer) drawBlock(dc *gg.Context, b seqlayout.Block) {
	dc.SetColor(r.theme.BlockStroke)
	dc.SetLineWidth(1)
	dc.SetDash()
	dc.DrawRectangle(b.X, b.Y, b.W, b.H)
	dc.Stroke()

	// label tab (top-left)
	tabW := r.measure(r.msgFace, b.Type) + 14
	tabH := 16.0
	dc.SetColor(r.theme.BlockLabelFill)
	dc.MoveTo(b.X, b.Y)
	dc.LineTo(b.X+tabW, b.Y)
	dc.LineTo(b.X+tabW, b.Y+tabH-5)
	dc.LineTo(b.X+tabW-5, b.Y+tabH)
	dc.LineTo(b.X, b.Y+tabH)
	dc.ClosePath()
	dc.FillPreserve()
	dc.SetColor(r.theme.BlockStroke)
	dc.Stroke()
	dc.SetFontFace(r.msgFace.FontFace())
	dc.SetColor(r.theme.BlockText)
	dc.DrawStringAnchored(b.Type, b.X+tabW/2, b.Y+tabH/2, 0.5, 0.5)

	// condition/label text centered at top
	if b.Label != "" {
		dc.SetColor(r.theme.BlockText)
		dc.DrawStringAnchored("["+b.Label+"]", b.X+b.W/2, b.Y+tabH/2, 0.5, 0.5)
	}

	// dividers (else/and/option) drawn dashed
	for _, dv := range b.Dividers {
		dc.SetColor(r.theme.BlockStroke)
		dc.SetDash(3, 3)
		dc.DrawLine(b.X, dv.Y, b.X+b.W, dv.Y)
		dc.Stroke()
		dc.SetDash()
		if dv.Label != "" {
			dc.SetColor(r.theme.BlockText)
			dc.DrawStringAnchored("["+dv.Label+"]", b.X+b.W/2, dv.Y+9, 0.5, 0.5)
		}
	}
}

func (r *Renderer) measure(f *textmetrics.Face, s string) float64 {
	w, _ := f.Measure(s)
	return w
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
