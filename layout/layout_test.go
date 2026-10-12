package layout

import (
	"os"
	"testing"

	"gmaid/parser"
)

func TestLayoutExamples(t *testing.T) {
	files := []string{
		"../testdata/flowchart-100.mmd",
		"../testdata/flowchart-130.mmd",
	}
	eng := NewEngine(DefaultConfig())
	for _, f := range files {
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		g, err := parser.Parse(string(src))
		if err != nil {
			t.Fatal(err)
		}
		dia := eng.Layout(g)
		t.Logf("%s: canvas=%.0fx%.0f nodes=%d subs=%d edges=%d", f, dia.Width, dia.Height, len(dia.Nodes), len(dia.Subs), len(dia.Edges))
		if dia.Width <= 0 || dia.Height <= 0 {
			t.Errorf("%s: non-positive canvas", f)
		}
		for _, n := range dia.Nodes {
			if n.X < 0 || n.Y < 0 {
				t.Errorf("%s: node %s negative pos %.1f,%.1f", f, n.ID, n.X, n.Y)
			}
			t.Logf("   node %-6s @ %.0f,%.0f %gx%g", n.ID, n.X, n.Y, n.W, n.H)
		}
		for _, s := range dia.Subs {
			t.Logf("   sub  %-10s @ %.0f,%.0f %.0fx%.0f", s.ID, s.X, s.Y, s.W, s.H)
		}
	}
}
