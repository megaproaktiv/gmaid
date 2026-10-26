// Package textmetrics provides font loading and string measurement shared by
// the layout and render stages so that node sizes computed during layout match
// what is actually drawn.
package textmetrics

import (
	"strings"
	"sync"

	"github.com/fogleman/gg"
	"github.com/golang/freetype/truetype"
	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goregular"
)

// Face wraps a font.Face at a given size with a measuring context.
type Face struct {
	face font.Face
	ctx  *gg.Context
	mu   sync.Mutex
}

var (
	regularFont *truetype.Font
	boldFont    *truetype.Font
	loadOnce    sync.Once
)

func loadFonts() {
	regularFont, _ = truetype.Parse(goregular.TTF)
	boldFont, _ = truetype.Parse(gobold.TTF)
}

// New returns a measuring Face for the given point size. If bold is true the
// bold variant of the built-in Go font is used.
func New(size float64, bold bool) *Face {
	loadOnce.Do(loadFonts)
	ft := regularFont
	if bold {
		ft = boldFont
	}
	face := truetype.NewFace(ft, &truetype.Options{Size: size})
	ctx := gg.NewContext(1, 1)
	ctx.SetFontFace(face)
	return &Face{face: face, ctx: ctx}
}

// FontFace exposes the underlying font.Face for drawing.
func (f *Face) FontFace() font.Face { return f.face }

// Measure returns the width and height of a single line of text.
func (f *Face) Measure(s string) (w, h float64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.ctx.MeasureString(s)
}

// MeasureLines returns the max line width and total height for multi-line text.
func (f *Face) MeasureLines(lines []string, lineHeight float64) (w, h float64) {
	for _, ln := range lines {
		lw, _ := f.Measure(ln)
		if lw > w {
			w = lw
		}
	}
	h = float64(len(lines)) * lineHeight
	return w, h
}

// SplitLines splits a label on explicit newlines produced by the parser.
func SplitLines(label string) []string {
	label = strings.ReplaceAll(label, "\r\n", "\n")
	parts := strings.Split(label, "\n")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		out = append(out, strings.TrimRight(p, " "))
	}
	if len(out) == 0 {
		out = []string{""}
	}
	return out
}

// WrapToWidth wraps each input line at word boundaries so no line exceeds
// maxW pixels when measured with f. Existing (explicit) line breaks are kept.
// A single word longer than maxW is left on its own line (not hard-split).
func (f *Face) WrapToWidth(lines []string, maxW float64) []string {
	if maxW <= 0 {
		return lines
	}
	var out []string
	for _, ln := range lines {
		words := strings.Fields(ln)
		if len(words) == 0 {
			out = append(out, "")
			continue
		}
		cur := words[0]
		for _, w := range words[1:] {
			cand := cur + " " + w
			if cw, _ := f.Measure(cand); cw <= maxW {
				cur = cand
			} else {
				out = append(out, cur)
				cur = w
			}
		}
		out = append(out, cur)
	}
	return out
}
