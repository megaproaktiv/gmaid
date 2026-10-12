//go:build ignore

// anonflow.go — robust flowchart label anonymiser. Parses each flowchart with
// the project's own parser to get the exact node labels, edge labels and
// subgraph titles, then replaces each literal label occurrence in the source
// with a length-preserved anonymisation drawn from the Hobbit vocabulary.
// Also strips `click` directives (URLs/tooltips) for thorough anonymisation.
//
// Run: go run anonflow.go <vocab.txt> <file.mmd...>
package main

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"gmaid/parser"
)

var (
	seed   uint64 = 0x9e3779b9
	byLen  map[int][]string
	lens   []int
	wordRe = regexp.MustCompile(`[A-Za-z][A-Za-z'\-]*`)
)

func rnd() uint64 { seed = seed*6364136223846793005 + 1442695040888963407; return seed >> 17 }

func loadVocab(path string) {
	b, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}
	byLen = map[int][]string{}
	for _, line := range strings.Split(string(b), "\n") {
		w := strings.TrimSpace(line)
		if w == "" || strings.HasPrefix(w, "#") || strings.Contains(w, "->") {
			continue
		}
		byLen[len(w)] = append(byLen[len(w)], w)
	}
	for l := range byLen {
		lens = append(lens, l)
	}
	sort.Ints(lens)
}

// pickLen returns a vocab word closest in length to n.
func pickLen(n int) string {
	for d := 0; d <= 14; d++ {
		for _, cand := range []int{n - d, n + d} {
			if cand >= 1 {
				if ws, ok := byLen[cand]; ok && len(ws) > 0 {
					return ws[rnd()%uint64(len(ws))]
				}
			}
		}
	}
	return "lorem"
}

// anonLabel replaces each alphabetic word in s with a length-matched vocab
// word, preserving <br/> tags, digits, punctuation and leading capitalization.
func anonLabel(s string) string {
	parts := regexp.MustCompile(`(?i)(<br\s*/?>)`).Split(s, -1)
	seps := regexp.MustCompile(`(?i)(<br\s*/?>)`).FindAllString(s, -1)
	var out strings.Builder
	for i, p := range parts {
		out.WriteString(wordRe.ReplaceAllStringFunc(p, func(w string) string {
			rep := pickLen(len(w))
			if len(w) > 0 && unicode.IsUpper(rune(w[0])) {
				rep = strings.ToUpper(rep[:1]) + rep[1:]
			}
			return rep
		}))
		if i < len(seps) {
			out.WriteString(seps[i])
		}
	}
	return out.String()
}

func main() {
	loadVocab(os.Args[1])
	for _, f := range os.Args[2:] {
		src, err := os.ReadFile(f)
		if err != nil {
			panic(err)
		}
		g, err := parser.Parse(string(src))
		if err != nil {
			fmt.Fprintln(os.Stderr, "parse", f, err)
			continue
		}
		// Collect distinct original label strings (longest first so that a
		// label that is a substring of another is replaced after it).
		labels := map[string]bool{}
		for _, n := range g.Nodes {
			if n.Label != "" && n.Label != n.ID {
				labels[n.Label] = true
			}
		}
		for _, e := range g.Edges {
			if e.Label != "" {
				labels[e.Label] = true
			}
		}
		for _, sg := range g.Subgraphs {
			if sg.Title != "" {
				labels[sg.Title] = true
			}
		}
		ordered := make([]string, 0, len(labels))
		for l := range labels {
			ordered = append(ordered, l)
		}
		sort.Slice(ordered, func(i, j int) bool { return len(ordered[i]) > len(ordered[j]) })

		text := string(src)
		for _, orig := range ordered {
			// The parser decodes <br/> and literal \n to a real newline in
			// labels; the source may use <br/>, <br>, <br /> or a literal \n.
			anon := anonLabel(orig)
			replaced := false
			for _, br := range []string{"<br/>", "<br>", "<br />", `\n`} {
				sf := strings.ReplaceAll(orig, "\n", br)
				if strings.Contains(text, sf) {
					text = strings.ReplaceAll(text, sf, strings.ReplaceAll(anon, "\n", br))
					replaced = true
				}
			}
			if !replaced && strings.Contains(text, orig) {
				text = strings.ReplaceAll(text, orig, anon)
			}
		}

		// Anonymise node IDs that look like real terms. IDs are structural, so
		// replace each whole-word occurrence consistently throughout the file
		// (declarations, edges, style, subgraph refs). Map each id to a stable
		// length-matched token; keep it alphanumeric and collision-free.
		idset := map[string]bool{}
		for id := range g.Nodes {
			idset[id] = true
		}
		for id := range g.Subgraphs {
			idset[id] = true
		}
		ids := make([]string, 0, len(idset))
		for id := range idset {
			ids = append(ids, id)
		}
		sort.Slice(ids, func(i, j int) bool { return len(ids[i]) > len(ids[j]) })
		idmap := map[string]string{}
		used := map[string]bool{}
		for _, id := range ids {
			base := pickLen(len(id))
			base = strings.Title(strings.ToLower(base))
			cand := base
			k := 1
			for used[cand] || cand == id {
				cand = fmt.Sprintf("%s%d", base, k)
				k++
			}
			used[cand] = true
			idmap[id] = cand
		}
		for _, id := range ids {
			re := regexp.MustCompile(`\b` + regexp.QuoteMeta(id) + `\b`)
			text = re.ReplaceAllString(text, idmap[id])
		}
		// Thorough anonymisation: drop click directives (URLs + tooltips).
		var kept []string
		sc := bufio.NewScanner(strings.NewReader(text))
		for sc.Scan() {
			line := sc.Text()
			if strings.HasPrefix(strings.TrimSpace(strings.ToLower(line)), "click ") {
				continue
			}
			kept = append(kept, line)
		}
		text = strings.Join(kept, "\n")
		if !strings.HasSuffix(text, "\n") {
			text += "\n"
		}
		if err := os.WriteFile(f, []byte(text), 0o644); err != nil {
			panic(err)
		}
	}
}
