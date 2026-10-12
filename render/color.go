package render

import (
	"image/color"
	"strings"
)

// parseHex parses CSS-ish color strings used in mermaid `style` directives:
//
//	#rgb, #rrggbb, #rrggbbaa, and a few common named colors.
//
// Returns ok=false for empty/unrecognized input so the caller can fall back to
// a theme default.
func parseHex(s string) (color.Color, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, false
	}
	if c, ok := namedColors[strings.ToLower(s)]; ok {
		return c, true
	}
	if !strings.HasPrefix(s, "#") {
		return nil, false
	}
	h := s[1:]
	switch len(h) {
	case 3: // #rgb
		r := hexNibble(h[0])
		g := hexNibble(h[1])
		b := hexNibble(h[2])
		if r < 0 || g < 0 || b < 0 {
			return nil, false
		}
		return color.RGBA{uint8(r * 17), uint8(g * 17), uint8(b * 17), 255}, true
	case 6: // #rrggbb
		r, ok1 := hexByte(h[0:2])
		g, ok2 := hexByte(h[2:4])
		b, ok3 := hexByte(h[4:6])
		if !(ok1 && ok2 && ok3) {
			return nil, false
		}
		return color.RGBA{r, g, b, 255}, true
	case 8: // #rrggbbaa
		r, ok1 := hexByte(h[0:2])
		g, ok2 := hexByte(h[2:4])
		b, ok3 := hexByte(h[4:6])
		a, ok4 := hexByte(h[6:8])
		if !(ok1 && ok2 && ok3 && ok4) {
			return nil, false
		}
		return color.RGBA{r, g, b, a}, true
	}
	return nil, false
}

func hexNibble(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'a' && c <= 'f':
		return int(c-'a') + 10
	case c >= 'A' && c <= 'F':
		return int(c-'A') + 10
	}
	return -1
}

func hexByte(s string) (uint8, bool) {
	hi := hexNibble(s[0])
	lo := hexNibble(s[1])
	if hi < 0 || lo < 0 {
		return 0, false
	}
	return uint8(hi*16 + lo), true
}

var namedColors = map[string]color.Color{
	"white":       color.RGBA{255, 255, 255, 255},
	"black":       color.RGBA{0, 0, 0, 255},
	"red":         color.RGBA{255, 0, 0, 255},
	"green":       color.RGBA{0, 128, 0, 255},
	"blue":        color.RGBA{0, 0, 255, 255},
	"yellow":      color.RGBA{255, 255, 0, 255},
	"orange":      color.RGBA{255, 165, 0, 255},
	"gray":        color.RGBA{128, 128, 128, 255},
	"grey":        color.RGBA{128, 128, 128, 255},
	"lightgrey":   color.RGBA{211, 211, 211, 255},
	"lightgray":   color.RGBA{211, 211, 211, 255},
	"transparent": color.RGBA{0, 0, 0, 0},
}
