package parser

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseAllFlowcharts(t *testing.T) {
	files, _ := filepath.Glob("../testdata/*.mmd")
	parsed := 0
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		src := string(b)
		trimmed := strings.TrimSpace(src)
		if !(strings.HasPrefix(trimmed, "flowchart") || strings.HasPrefix(trimmed, "graph")) {
			continue
		}
		g, err := Parse(src)
		if err != nil {
			t.Errorf("parse %s: %v", f, err)
			continue
		}
		if len(g.Nodes) == 0 {
			t.Errorf("%s: no nodes parsed", filepath.Base(f))
		}
		parsed++
	}
	t.Logf("parsed %d flowchart files", parsed)
}

func TestParseExamples(t *testing.T) {
	files := []string{
		"../testdata/430-work-package-wismo-shipping-status-01.mmd",
		"../testdata/503-adr-data-warehouse-integration-01.mmd",
	}
	for _, f := range files {
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		g, err := Parse(string(src))
		if err != nil {
			t.Fatalf("parse %s: %v", f, err)
		}
		t.Logf("%s: dir=%s nodes=%d edges=%d subgraphs=%d", f, g.Direction, len(g.Nodes), len(g.Edges), len(g.Subgraphs))
		for _, id := range g.NodeOrder {
			n := g.Nodes[id]
			t.Logf("  node %-6s shape=%d parent=%-10s style=%v label=%q", n.ID, n.Shape, n.Parent, n.Style, n.Label)
		}
		for _, e := range g.Edges {
			t.Logf("  edge %s -> %s line=%d arrow=%d label=%q", e.From, e.To, e.Line, e.Arrow, e.Label)
		}
	}
}
