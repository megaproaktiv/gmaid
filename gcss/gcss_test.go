package gcss

import (
	"image/color"
	"testing"
)

func TestMatchOperators(t *testing.T) {
	css := `
	/* role colors */
	rect[id$="-wp4_ai"]  { fill:#d62728; stroke:#7a1516; }
	rect[id*="-engc_"]   { fill:#2ca02c !important; }
	rect[id^="my-svg-x"] { fill:#111111; }
	rect[id="exact"]     { fill:#222222; }
	`
	s := Parse(css)

	// suffix
	f, st, hf, hs, ok := s.Match("wp4_ai")
	if !ok || !hf || !hs {
		t.Fatalf("wp4_ai should match fill+stroke, got ok=%v hf=%v hs=%v", ok, hf, hs)
	}
	if f.(color.RGBA) != (color.RGBA{0xd6, 0x27, 0x28, 255}) {
		t.Errorf("wp4_ai fill = %v", f)
	}
	if st.(color.RGBA) != (color.RGBA{0x7a, 0x15, 0x16, 255}) {
		t.Errorf("wp4_ai stroke = %v", st)
	}

	// substring (matches engc_01, engc_12, ...)
	if f, _, hf, _, ok := s.Match("engc_01"); !ok || !hf || f.(color.RGBA) != (color.RGBA{0x2c, 0xa0, 0x2c, 255}) {
		t.Errorf("engc_01 fill = %v ok=%v", f, ok)
	}

	// prefix
	if _, _, _, _, ok := s.Match("x_task"); !ok {
		t.Errorf("x_task should match prefix rule")
	}

	// exact
	if _, _, _, _, ok := s.Match("exact"); !ok {
		t.Errorf("exact should match")
	}

	// no match
	if _, _, _, _, ok := s.Match("nope"); ok {
		t.Errorf("nope should not match")
	}
}

func TestImportantWins(t *testing.T) {
	// non-important later rule vs important earlier rule: important wins.
	css := `
	rect[id$="-t"] { fill:#111111 !important; }
	rect[id$="-t"] { fill:#222222; }
	`
	s := Parse(css)
	f, _, _, _, _ := s.Match("t")
	if f.(color.RGBA) != (color.RGBA{0x11, 0x11, 0x11, 255}) {
		t.Errorf("important should win, got %v", f)
	}
}

func TestSourceOrderTie(t *testing.T) {
	// equal priority: later wins.
	css := `
	rect[id$="-t"] { fill:#111111; }
	rect[id$="-t"] { fill:#222222; }
	`
	s := Parse(css)
	f, _, _, _, _ := s.Match("t")
	if f.(color.RGBA) != (color.RGBA{0x22, 0x22, 0x22, 255}) {
		t.Errorf("later rule should win, got %v", f)
	}
}
