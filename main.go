// Command gmaid renders mermaid flowchart .mmd files to PNG images using a
// pure-Go pipeline (parser -> layered layout -> gg rasterizer). No browser or
// cgo is required.
//
// Usage:
//
//	gmaid [flags] [paths...]
//
// Each path may be a .mmd file or a directory. Directories are scanned for
// .mmd files whose content begins with "flowchart" or "graph". For every
// matching file FOO.mmd a sibling FOO.png is written.
//
// With no paths, the ./testdata directory is used.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/megaproaktiv/gmaid/glayout"
	"github.com/megaproaktiv/gmaid/gparser"
	"github.com/megaproaktiv/gmaid/grender"
	"github.com/megaproaktiv/gmaid/layout"
	"github.com/megaproaktiv/gmaid/parser"
	"github.com/megaproaktiv/gmaid/render"
	"github.com/megaproaktiv/gmaid/seqlayout"
	"github.com/megaproaktiv/gmaid/seqparser"
	"github.com/megaproaktiv/gmaid/seqrender"
)

// Build information, injected via -ldflags by GoReleaser.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	scale := flag.Float64("scale", 2.0, "output resolution scale factor")
	outDir := flag.String("out", "output", "output directory for rendered PNGs (batch mode)")
	verbose := flag.Bool("v", false, "verbose logging")
	showVersion := flag.Bool("version", false, "print version and exit")
	// mmdc-compatible flags so gmaid is a drop-in replacement for `mmdc -i ...`.
	inFile := flag.String("i", "", "input .mmd file (mmdc-compatible)")
	outFile := flag.String("o", "", "output .png file (mmdc-compatible)")
	_ = flag.String("b", "", "background (accepted for mmdc compatibility, ignored)")
	flag.Parse()

	if *showVersion {
		fmt.Printf("gmaid %s (commit %s, built %s)\n", version, commit, date)
		return
	}

	rr := &renderers{
		fcEng:  layout.NewEngine(layout.DefaultConfig()),
		fcRnd:  render.New(render.DefaultTheme(), layout.DefaultConfig(), *scale),
		seqEng: seqlayout.NewEngine(seqlayout.DefaultConfig()),
		seqRnd: seqrender.New(seqrender.DefaultTheme(), seqlayout.DefaultConfig(), *scale),
		gEng:   glayout.NewEngine(glayout.DefaultConfig()),
		gRnd:   grender.New(grender.DefaultTheme(), glayout.DefaultConfig(), *scale),
	}

	// mmdc-compatible single-file mode: `gmaid -i input.mmd [-o output.png]`.
	if *inFile != "" {
		out := *outFile
		if out == "" {
			out = strings.TrimSuffix(*inFile, filepath.Ext(*inFile)) + ".png"
		}
		if dir := filepath.Dir(out); dir != "" {
			_ = os.MkdirAll(dir, 0o755)
		}
		if err := rr.renderTo(*inFile, out); err != nil {
			fmt.Fprintf(os.Stderr, "FAIL %s: %v\n", *inFile, err)
			os.Exit(1)
		}
		fmt.Println(out)
		return
	}

	// Batch mode: render every supported .mmd under the given paths (default
	// ./testdata) into the output directory.
	paths := flag.Args()
	if len(paths) == 0 {
		paths = []string{"testdata"}
	}

	files, err := collectFiles(paths)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	if len(files) == 0 {
		fmt.Fprintln(os.Stderr, "no supported .mmd files found")
		os.Exit(1)
	}

	if *outDir != "" {
		if err := os.MkdirAll(*outDir, 0o755); err != nil {
			fmt.Fprintln(os.Stderr, "error: cannot create output dir:", err)
			os.Exit(1)
		}
	}

	ok, fail := 0, 0
	for _, f := range files {
		out, err := rr.renderFile(f, *outDir)
		if err != nil {
			fail++
			fmt.Fprintf(os.Stderr, "FAIL %s: %v\n", f, err)
			continue
		}
		ok++
		if *verbose {
			fmt.Printf("ok   %s -> %s\n", f, out)
		} else {
			fmt.Printf("%s\n", out)
		}
	}
	fmt.Fprintf(os.Stderr, "rendered %d file(s), %d failed\n", ok, fail)
	if fail > 0 {
		os.Exit(1)
	}
}

// renderers bundles the per-diagram-type engines and rasterizers.
type renderers struct {
	fcEng  *layout.Engine
	fcRnd  *render.Renderer
	seqEng *seqlayout.Engine
	seqRnd *seqrender.Renderer
	gEng   *glayout.Engine
	gRnd   *grender.Renderer
}

// renderFile renders a single .mmd into outDir, deriving the PNG name from the
// source. It is the batch-mode entry point.
func (rr *renderers) renderFile(path, outDir string) (string, error) {
	out := outputPath(path, outDir)
	if err := rr.renderTo(path, out); err != nil {
		return "", err
	}
	return out, nil
}

// renderTo parses, lays out and renders a single .mmd file to the exact output
// path, dispatching by the diagram's first keyword. The flowchart and sequence
// pipelines are left exactly as they were.
func (rr *renderers) renderTo(path, out string) error {
	src, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	switch diagramKind(string(src)) {
	case kindGantt:
		c, err := gparser.Parse(string(src))
		if err != nil {
			return fmt.Errorf("gantt parse: %w", err)
		}
		if err := rr.gRnd.Render(rr.gEng.Layout(c), out); err != nil {
			return fmt.Errorf("gantt render: %w", err)
		}
		return nil
	case kindSequence:
		d, err := seqparser.Parse(string(src))
		if err != nil {
			return fmt.Errorf("seq parse: %w", err)
		}
		if err := rr.seqRnd.Render(rr.seqEng.Layout(d), out); err != nil {
			return fmt.Errorf("seq render: %w", err)
		}
		return nil
	case kindFlowchart:
		g, err := parser.Parse(string(src))
		if err != nil {
			return fmt.Errorf("parse: %w", err)
		}
		if err := rr.fcRnd.Render(rr.fcEng.Layout(g), out); err != nil {
			return fmt.Errorf("render: %w", err)
		}
		return nil
	default:
		return fmt.Errorf("unsupported diagram type")
	}
}

// outputPath maps SRC.mmd to SRC.png, optionally in outDir.
func outputPath(src, outDir string) string {
	base := strings.TrimSuffix(filepath.Base(src), filepath.Ext(src)) + ".png"
	if outDir != "" {
		return filepath.Join(outDir, base)
	}
	return filepath.Join(filepath.Dir(src), base)
}

// collectFiles expands the given paths into a list of supported .mmd files
// (flowchart or sequence diagrams).
func collectFiles(paths []string) ([]string, error) {
	var out []string
	seen := map[string]bool{}
	add := func(p string) {
		if !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			return nil, err
		}
		if info.IsDir() {
			matches, _ := filepath.Glob(filepath.Join(p, "*.mmd"))
			for _, m := range matches {
				b, _ := os.ReadFile(m)
				if diagramKind(string(b)) != kindUnknown {
					add(m)
				}
			}
			continue
		}
		if strings.EqualFold(filepath.Ext(p), ".mmd") {
			b, _ := os.ReadFile(p)
			if diagramKind(string(b)) != kindUnknown {
				add(p)
			} else {
				fmt.Fprintf(os.Stderr, "skip %s: unsupported diagram type\n", p)
			}
		}
	}
	return out, nil
}

// diagramKind classifies a diagram by its first keyword. Dispatch and the
// flowchart-only guarantee both rely on this exact-first-word matching.
type kind int

const (
	kindUnknown kind = iota
	kindFlowchart
	kindSequence
	kindGantt
)

func diagramKind(src string) kind {
	switch firstKeyword(src) {
	case "flowchart":
		return kindFlowchart
	case "sequenceDiagram":
		return kindSequence
	case "gantt":
		return kindGantt
	default:
		return kindUnknown
	}
}

// firstKeyword returns the first word of the first non-empty, non-comment,
// non-front-matter line.
func firstKeyword(src string) string {
	for _, line := range strings.Split(src, "\n") {
		t := strings.TrimSpace(line)
		if t == "" || strings.HasPrefix(t, "%%") || strings.HasPrefix(t, "---") {
			continue
		}
		fields := strings.Fields(t)
		if len(fields) == 0 {
			return ""
		}
		return fields[0]
	}
	return ""
}

// isFlowchart reports whether a file is a flowchart diagram (first word
// "flowchart"). Retained as a small readable predicate; the main dispatch uses
// diagramKind directly.
func isFlowchart(path string) bool {
	b, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return diagramKind(string(b)) == kindFlowchart
}

var _ = isFlowchart
