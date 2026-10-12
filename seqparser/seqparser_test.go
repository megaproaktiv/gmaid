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
	b, _ := os.ReadFile("../testdata/sequence-260.mmd")
	d, err := Parse(string(b))
	if err != nil {
		t.Fatal(err)
	}
	// sequence-260 is a stable fixture (anonymised but structurally unchanged):
	// autonumber on, 5 participants, 15 messages, 1 note, 3 block-events
	// (alt/else/end). Assert these so a parse regression is caught.
	if !d.Autonumber {
		t.Errorf("autonumber: got false, want true")
	}
	if len(d.Participants) != 5 {
		t.Errorf("participants: got %d, want 5", len(d.Participants))
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
	if msgs != 15 {
		t.Errorf("messages: got %d, want 15", msgs)
	}
	if notes != 1 {
		t.Errorf("notes: got %d, want 1", notes)
	}
	if blocks != 3 {
		t.Errorf("block-events: got %d, want 3", blocks)
	}
	t.Logf("autonumber=%v participants=%d messages=%d notes=%d block-events=%d",
		d.Autonumber, len(d.Participants), msgs, notes, blocks)
}
