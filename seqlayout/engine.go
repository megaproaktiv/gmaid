package seqlayout

import (
	"gmaid/seqmodel"
	"gmaid/textmetrics"
)

// Engine lays out sequence diagrams.
type Engine struct {
	cfg   Config
	face  *textmetrics.Face
	bold  *textmetrics.Face
	small *textmetrics.Face
}

// NewEngine builds an engine with the given config.
func NewEngine(cfg Config) *Engine {
	return &Engine{
		cfg:   cfg,
		face:  textmetrics.New(cfg.FontSize, false),
		bold:  textmetrics.New(cfg.FontSize, true),
		small: textmetrics.New(cfg.LabelFont, false),
	}
}

// Layout computes geometry for the whole diagram.
func (e *Engine) Layout(d *seqmodel.Diagram) *Diagram {
	out := &Diagram{}
	cfg := e.cfg

	// --- 1. Actor box widths from labels ---
	n := len(d.Participants)
	if n == 0 {
		out.Width, out.Height = 1, 1
		return out
	}
	actorW := make([]float64, n)
	actorLines := make([][]string, n)
	for i, p := range d.Participants {
		lines := textmetrics.SplitLines(p.Label)
		actorLines[i] = lines
		w := 0.0
		for _, ln := range lines {
			lw, _ := e.bold.Measure(ln)
			if lw > w {
				w = lw
			}
		}
		w += 2 * cfg.ActorPadX
		if w < cfg.ActorW {
			w = cfg.ActorW
		}
		actorW[i] = w
	}

	// --- 2. Column center X positions ---
	idx := func(id string) int { return d.Index(id) }

	// --- 2. Label-proportional inter-column spacing ---
	// centerGap[i] is the required CENTER-to-center distance between lifeline i
	// and i+1. A message label sits centered between the two lifelines, so the
	// only thing that must fit there is the label text plus its side padding —
	// NOT the actor-box half-widths (the label may overhang the boxes). Using
	// center distance avoids the boxes silently widening the gap.
	centerGap := make([]float64, n) // index i = center distance col i -> i+1
	spaceW, _ := e.small.Measure(" ")
	pad := 4 * spaceW // 2 spaces left + 2 spaces right
	// minimum center distance that keeps adjacent boxes from overlapping
	minCenter := func(i int) float64 {
		return actorW[i]/2 + cfg.ActorGap + actorW[i+1]/2
	}
	for i := 0; i < n-1; i++ {
		centerGap[i] = minCenter(i)
	}
	selfExtra := make([]float64, n) // extra width to the right for self-loops

	for _, ev := range d.Events {
		switch ev.Kind {
		case seqmodel.EvMessage:
			fi, ti := idx(ev.From), idx(ev.To)
			if fi < 0 || ti < 0 {
				continue
			}
			lw := e.measureMaxLine(e.small, textmetrics.SplitLines(ev.Text))
			if fi == ti {
				// self message: reserve room to the right of this column
				need := lw + cfg.SelfMsgGap + 20
				if need > selfExtra[fi] {
					selfExtra[fi] = need
				}
				continue
			}
			lo, hi := fi, ti
			if hi < lo {
				lo, hi = hi, lo
			}
			nb := hi - lo // number of boundaries spanned
			per := (lw + pad) / float64(nb)
			for b := lo; b < hi; b++ {
				if per > centerGap[b] {
					centerGap[b] = per
				}
			}
		case seqmodel.EvNote:
			if len(ev.Actors) == 0 {
				continue
			}
			lw := e.measureMaxLine(e.face, textmetrics.SplitLines(ev.Text)) + 2*cfg.NotePadX
			a0 := idx(ev.Actors[0])
			if len(ev.Actors) >= 2 {
				a1 := idx(ev.Actors[len(ev.Actors)-1])
				lo, hi := a0, a1
				if hi < lo {
					lo, hi = hi, lo
				}
				nb := hi - lo
				if nb > 0 {
					per := lw / float64(nb)
					for b := lo; b < hi; b++ {
						if per > centerGap[b] {
							centerGap[b] = per
						}
					}
				}
			}
		}
	}

	// Place columns left to right using the per-boundary center distances and
	// self-loop clearances.
	cx := make([]float64, n)
	x := cfg.Margin
	for i := 0; i < n; i++ {
		cx[i] = x + actorW[i]/2
		if i < n-1 {
			g := centerGap[i]
			// a self-loop on column i needs clearance before the next column
			if need := actorW[i]/2 + selfExtra[i] + 40 + actorW[i+1]/2; need > g {
				g = need
			}
			x += g
		}
	}
	// Safety net: guarantee adjacent boxes never overlap, regardless of the
	// label-driven center distances above. Each box pair must be at least
	// (halfWidth_i + ActorGap + halfWidth_{i+1}) apart center-to-center.
	for i := 0; i < n-1; i++ {
		need := actorW[i]/2 + cfg.ActorGap + actorW[i+1]/2
		if cx[i+1]-cx[i] < need {
			shift := need - (cx[i+1] - cx[i])
			for j := i + 1; j < n; j++ {
				cx[j] += shift
			}
		}
	}
	// widen() enforces a minimum center distance (used for note-over-single).
	widen := func(i int, need float64) {
		if i >= 0 && i < n-1 && cx[i+1]-cx[i] < need {
			shift := need - (cx[i+1] - cx[i])
			for j := i + 1; j < n; j++ {
				cx[j] += shift
			}
		}
	}
	for _, ev := range d.Events {
		if ev.Kind == seqmodel.EvNote && len(ev.Actors) == 1 {
			lw := e.measureMaxLine(e.face, textmetrics.SplitLines(ev.Text)) + 2*cfg.NotePadX
			ai := idx(ev.Actors[0])
			if ai >= 0 && ai < n-1 && lw > actorW[ai] {
				widen(ai, lw)
			}
		}
	}

	// --- 3. Walk events top-to-bottom assigning Y ---
	out.LifelineTop = cfg.Margin + cfg.ActorH + cfg.TopGap
	y := out.LifelineTop
	var blockStack []int // indices into out.Blocks
	seqNum := 0

	for _, ev := range d.Events {
		switch ev.Kind {
		case seqmodel.EvMessage:
			fi, ti := idx(ev.From), idx(ev.To)
			if fi < 0 || ti < 0 {
				continue
			}
			lines := textmetrics.SplitLines(ev.Text)
			self := fi == ti
			y += cfg.MsgGap
			num := 0
			if d.Autonumber {
				seqNum++
				num = seqNum
			}
			labelH := float64(len(lines)) * cfg.LineHeight
			m := Message{
				Ev: ev, FromX: cx[fi], ToX: cx[ti], Self: self, Number: num,
				Lines: lines, LabelY: y, Y: y + labelH + 4,
			}
			if self {
				m.Y = y + labelH + 4
				y = m.Y + cfg.SelfMsgGap
			} else {
				y = m.Y + 6
			}
			out.Messages = append(out.Messages, m)
		case seqmodel.EvNote:
			if len(ev.Actors) == 0 {
				continue
			}
			lines := textmetrics.SplitLines(ev.Text)
			tw := e.measureMaxLine(e.face, lines) + 2*cfg.NotePadX
			th := float64(len(lines))*cfg.LineHeight + 2*cfg.NotePadY
			var nx, nw float64
			a0 := idx(ev.Actors[0])
			switch ev.Placement {
			case seqmodel.NoteOver:
				if len(ev.Actors) >= 2 {
					a1 := idx(ev.Actors[len(ev.Actors)-1])
					lo, hi := cx[a0], cx[a1]
					if hi < lo {
						lo, hi = hi, lo
					}
					nx = lo - 20
					nw = (hi - lo) + 40
					if nw < tw {
						mid := (lo + hi) / 2
						nx = mid - tw/2
						nw = tw
					}
				} else {
					nw = tw
					nx = cx[a0] - tw/2
				}
			case seqmodel.NoteRightOf:
				nw = tw
				nx = cx[a0] + 10
			case seqmodel.NoteLeftOf:
				nw = tw
				nx = cx[a0] - 10 - tw
			}
			y += cfg.NoteGap
			out.Notes = append(out.Notes, NoteBox{Ev: ev, X: nx, Y: y, W: nw, H: th, Lines: lines})
			y += th + cfg.NoteGap
		case seqmodel.EvBlockStart:
			y += cfg.BlockGap
			b := Block{Type: ev.BlockType, Label: ev.BlockText, Color: ev.BlockColor, Y: y, Depth: len(blockStack)}
			out.Blocks = append(out.Blocks, b)
			blockStack = append(blockStack, len(out.Blocks)-1)
			y += cfg.BlockPadTop
		case seqmodel.EvBlockElse:
			if len(blockStack) > 0 {
				y += cfg.BlockGap
				bi := blockStack[len(blockStack)-1]
				out.Blocks[bi].Dividers = append(out.Blocks[bi].Dividers, Divider{Y: y, Label: ev.BlockText})
				y += cfg.BlockPadTop * 0.6
			}
		case seqmodel.EvBlockEnd:
			if len(blockStack) > 0 {
				y += cfg.BlockGap
				bi := blockStack[len(blockStack)-1]
				blockStack = blockStack[:len(blockStack)-1]
				e.finalizeBlock(&out.Blocks[bi], cx, actorW, y)
				y += cfg.BlockGap
			}
		}
	}

	y += cfg.MsgGap
	out.LifelineBottom = y

	// Clamp any note that would extend past the left margin back to the margin
	// (mermaid keeps notes inside the canvas). We nudge just the note rather
	// than shifting the whole diagram, so overall width/aspect is unaffected.
	for i := range out.Notes {
		if out.Notes[i].X < cfg.Margin {
			out.Notes[i].X = cfg.Margin
		}
	}

	// --- 4. Build actors (top + mirrored bottom handled by renderer) ---
	for i, p := range d.Participants {
		out.Actors = append(out.Actors, Actor{
			ID: p.ID, Label: p.Label, Kind: p.Kind,
			CX: cx[i], W: actorW[i], Lines: actorLines[i],
		})
	}

	// --- 5. Canvas size ---
	maxX := 0.0
	for i := 0; i < n; i++ {
		if r := cx[i] + actorW[i]/2; r > maxX {
			maxX = r
		}
	}
	for _, nb := range out.Notes {
		if r := nb.X + nb.W; r > maxX {
			maxX = r
		}
	}
	for _, b := range out.Blocks {
		if r := b.X + b.W; r > maxX {
			maxX = r
		}
	}
	for _, m := range out.Messages {
		if m.Self {
			if r := m.FromX + cfg.SelfMsgGap + e.measureMaxLine(e.small, m.Lines) + 10; r > maxX {
				maxX = r
			}
		}
	}
	out.Width = maxX + cfg.Margin
	out.Height = out.LifelineBottom + cfg.ActorH + cfg.Margin
	return out
}

// finalizeBlock sets a block's x/width to span all columns it covers and its
// height to reach the current y. For simplicity blocks span all participants
// (mermaid narrows to involved actors, refined in later iterations).
func (e *Engine) finalizeBlock(b *Block, cx, actorW []float64, endY float64) {
	n := len(cx)
	left := cx[0] - actorW[0]/2 - e.cfg.BlockPadX - float64(b.Depth)*4
	right := cx[n-1] + actorW[n-1]/2 + e.cfg.BlockPadX + float64(b.Depth)*4
	b.X = left
	b.W = right - left
	b.H = endY - b.Y
}

func (e *Engine) measureMaxLine(f *textmetrics.Face, lines []string) float64 {
	w := 0.0
	for _, ln := range lines {
		lw, _ := f.Measure(ln)
		if lw > w {
			w = lw
		}
	}
	return w
}

// sumHalfWidths returns the actor-box width that a note spanning columns
// lo..hi already covers via the half-boxes at each end.
func sumHalfWidths(actorW []float64, lo, hi int) float64 {
	if lo < 0 || hi >= len(actorW) || hi <= lo {
		return 0
	}
	return actorW[lo]/2 + actorW[hi]/2
}

var _ = sumHalfWidths // retained helper; not currently referenced
