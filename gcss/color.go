package gcss

import (
	"image/color"
	"strings"
)

// ParseColor parses #rgb, #rrggbb, #rrggbbaa and a few named colors used in the
// gantt stylesheet. Returns ok=false for unrecognized input.
//
// NOTE: Go's image/color.RGBA is alpha-premultiplied. For #rrggbbaa with a<255
// we premultiply so the stored color is valid (a translucent tint rather than
// an invalid/over-bright value).
func ParseColor(s string) (color.Color, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, false
	}
	if c, ok := named[strings.ToLower(s)]; ok {
		return c, true
	}
	if !strings.HasPrefix(s, "#") {
		return nil, false
	}
	h := s[1:]
	switch len(h) {
	case 3:
		r, g, b := nib(h[0]), nib(h[1]), nib(h[2])
		if r < 0 || g < 0 || b < 0 {
			return nil, false
		}
		return color.RGBA{uint8(r * 17), uint8(g * 17), uint8(b * 17), 255}, true
	case 6:
		r, ok1 := b2(h[0:2])
		g, ok2 := b2(h[2:4])
		b, ok3 := b2(h[4:6])
		if !(ok1 && ok2 && ok3) {
			return nil, false
		}
		return color.RGBA{r, g, b, 255}, true
	case 8:
		r, ok1 := b2(h[0:2])
		g, ok2 := b2(h[2:4])
		b, ok3 := b2(h[4:6])
		a, ok4 := b2(h[6:8])
		if !(ok1 && ok2 && ok3 && ok4) {
			return nil, false
		}
		// premultiply to keep RGBA valid
		pm := func(v uint8) uint8 { return uint8(uint16(v) * uint16(a) / 255) }
		return color.RGBA{pm(r), pm(g), pm(b), a}, true
	}
	return nil, false
}

func nib(c byte) int {
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

func b2(s string) (uint8, bool) {
	hi, lo := nib(s[0]), nib(s[1])
	if hi < 0 || lo < 0 {
		return 0, false
	}
	return uint8(hi*16 + lo), true
}

var named = map[string]color.Color{
	"white": color.RGBA{255, 255, 255, 255},
	"black": color.RGBA{0, 0, 0, 255},
	"red":   color.RGBA{255, 0, 0, 255},
	"green": color.RGBA{0, 128, 0, 255},
	"blue":  color.RGBA{0, 0, 255, 255},
	"grey":  color.RGBA{128, 128, 128, 255},
	"gray":  color.RGBA{128, 128, 128, 255},
}
