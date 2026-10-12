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

	"gmaid/layout"
	"gmaid/parser"
	"gmaid/render"
)

func main() {
	scale := flag.Float64("scale", 2.0, "output resolution scale factor")
	outDir := flag.String("out", "output", "output directory for rendered PNGs")
	verbose := flag.Bool("v", false, "verbose logging")
	flag.Parse()

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
		fmt.Fprintln(os.Stderr, "no flowchart .mmd files found")
		os.Exit(1)
	}

	if *outDir != "" {
		if err := os.MkdirAll(*outDir, 0o755); err != nil {
			fmt.Fprintln(os.Stderr, "error: cannot create output dir:", err)
			os.Exit(1)
		}
	}

	cfg := layout.DefaultConfig()
	eng := layout.NewEngine(cfg)
	rnd := render.New(render.DefaultTheme(), cfg, *scale)

	ok, fail := 0, 0
	for _, f := range files {
		out, err := renderFile(eng, rnd, f, *outDir)
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

// renderFile parses, lays out and renders a single .mmd file, returning the
// output PNG path.
func renderFile(eng *layout.Engine, rnd *render.Renderer, path, outDir string) (string, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	g, err := parser.Parse(string(src))
	if err != nil {
		return "", fmt.Errorf("parse: %w", err)
	}
	dia := eng.Layout(g)
	out := outputPath(path, outDir)
	if err := rnd.Render(dia, out); err != nil {
		return "", fmt.Errorf("render: %w", err)
	}
	return out, nil
}

// outputPath maps SRC.mmd to SRC.png, optionally in outDir.
func outputPath(src, outDir string) string {
	base := strings.TrimSuffix(filepath.Base(src), filepath.Ext(src)) + ".png"
	if outDir != "" {
		return filepath.Join(outDir, base)
	}
	return filepath.Join(filepath.Dir(src), base)
}

// collectFiles expands the given paths into a list of flowchart .mmd files.
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
				if isFlowchart(m) {
					add(m)
				}
			}
			continue
		}
		// explicit file: include regardless, but still require flowchart header
		if strings.EqualFold(filepath.Ext(p), ".mmd") {
			if isFlowchart(p) {
				add(p)
			} else {
				fmt.Fprintf(os.Stderr, "skip %s: not a flowchart diagram\n", p)
			}
		}
	}
	return out, nil
}

// isFlowchart reports whether a file is a flowchart diagram: its first
// non-empty, non-comment line must have "flowchart" as its exact first word
// (e.g. "flowchart TD"). The "graph" alias and any other diagram type are
// intentionally excluded, as are words like "flowcharts".
func isFlowchart(path string) bool {
	b, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(b), "\n") {
		t := strings.TrimSpace(line)
		if t == "" || strings.HasPrefix(t, "%%") || strings.HasPrefix(t, "---") {
			continue
		}
		fields := strings.Fields(t)
		return len(fields) > 0 && fields[0] == "flowchart"
	}
	return false
}
