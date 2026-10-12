package seqparser

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseAllSequence(t *testing.T) {
	files, _ := filepath.Glob("../testdata/*.mmd")
	n := 0
	for _, f := range files {
		b, _ := os.ReadFile(f)
		if !strings.HasPrefix(strings.TrimSpace(string(b)), "sequenceDiagram") {
			continue
		}
		d, err := Parse(string(b))
		if err != nil {
			t.Errorf("%s: %v", f, err)
			continue
		}
		if len(d.Participants) == 0 {
			t.Errorf("%s: no participants", filepath.Base(f))
		}
		n++
	}
	t.Logf("parsed %d sequence files", n)
}

func TestParseExample(t *testing.T) {
	b, _ := os.ReadFile("../testdata/600-email-contact-flow-02.mmd")
	d, err := Parse(string(b))
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("autonumber=%v participants=%d events=%d", d.Autonumber, len(d.Participants), len(d.Events))
	for _, p := range d.Participants {
		t.Logf("  participant %s = %q", p.ID, p.Label)
	}
	msgs, notes, blocks := 0, 0, 0
	for _, e := range d.Events {
		switch e.Kind {
		case 0:
			msgs++
		case 1:
			notes++
		case 2, 3, 4:
			blocks++
		}
	}
	t.Logf("  messages=%d notes=%d block-events=%d", msgs, notes, blocks)
}
