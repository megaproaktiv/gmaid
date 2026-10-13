// Package parser turns mermaid flowchart source text into a model.Graph.
//
// It supports the subset of the flowchart grammar found in the project's
// testdata: directions TD/TB/LR/BT/RL, node shapes (rect, round, stadium,
// subroutine, cylinder, circle, diamond, hexagon, parallelogram, trapezoid),
// edges (solid/dotted/thick/invisible with optional labels and arrow/open/
// circle/cross heads), subgraphs with nested direction, and `style`
// directives. `click` lines and `%%` comments are ignored.
package parser

import (
	"bufio"
	"strings"

	"github.com/megaproaktiv/gmaid/model"
)

// Parse reads mermaid source and returns the populated graph.
func Parse(src string) (*model.Graph, error) {
	g := model.NewGraph()
	p := &parser{g: g}
	sc := bufio.NewScanner(strings.NewReader(src))
	sc.Buffer(make([]byte, 0, 1024*1024), 1024*1024)
	for sc.Scan() {
		p.line(sc.Text())
	}
	return g, sc.Err()
}

type parser struct {
	g          *model.Graph
	headerSeen bool
	// stack of subgraph ids currently open ("" sentinel is never pushed)
	stack []string
}

func (p *parser) curParent() string {
	if len(p.stack) == 0 {
		return ""
	}
	return p.stack[len(p.stack)-1]
}

func (p *parser) line(raw string) {
	line := strings.TrimSpace(stripComment(raw))
	if line == "" {
		return
	}

	// Header: `flowchart TD` / `graph LR`
	if !p.headerSeen {
		if kw, dir, ok := matchHeader(line); ok {
			_ = kw
			if dir != "" {
				p.g.Direction = dir
			}
			p.headerSeen = true
			return
		}
		// Some files might have front matter / config; skip until header.
		if strings.HasPrefix(line, "---") || strings.HasPrefix(line, "config") {
			return
		}
	}

	lower := strings.ToLower(line)

	switch {
	case strings.HasPrefix(lower, "subgraph "):
		p.openSubgraph(line)
		return
	case lower == "end":
		p.closeSubgraph()
		return
	case strings.HasPrefix(lower, "direction "):
		p.setDirection(line)
		return
	case strings.HasPrefix(lower, "click "):
		return // interaction, ignored for rendering
	case strings.HasPrefix(lower, "style "):
		p.parseStyle(line)
		return
	case strings.HasPrefix(lower, "classdef "), strings.HasPrefix(lower, "class "),
		strings.HasPrefix(lower, "linkstyle "):
		return // not needed for the target files; safely ignored
	}

	// Otherwise it is a statement line: either an edge chain or a bare node.
	p.parseStatement(line)
}

// setDirection applies a `direction XX` statement to the current subgraph
// (or to the whole graph if at top level).
func (p *parser) setDirection(line string) {
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return
	}
	dir := normalizeDir(fields[1])
	if dir == "" {
		return
	}
	if cur := p.curParent(); cur != "" {
		if sg := p.g.Subgraphs[cur]; sg != nil {
			sg.Direction = dir
		}
	} else {
		p.g.Direction = dir
	}
}

// openSubgraph parses `subgraph ID["Title"]` or `subgraph Title`.
func (p *parser) openSubgraph(line string) {
	rest := strings.TrimSpace(line[len("subgraph"):])
	id, title := parseSubgraphHeader(rest)

	sg := &model.Subgraph{ID: id, Title: title, Parent: p.curParent()}
	if _, exists := p.g.Subgraphs[id]; !exists {
		p.g.Subgraphs[id] = sg
		p.g.SubOrder = append(p.g.SubOrder, id)
	} else {
		sg = p.g.Subgraphs[id]
	}
	// register as child of parent
	if par := p.curParent(); par != "" {
		if psg := p.g.Subgraphs[par]; psg != nil {
			psg.Children = appendUnique(psg.Children, id)
		}
	}
	p.stack = append(p.stack, id)
}

func (p *parser) closeSubgraph() {
	if len(p.stack) > 0 {
		p.stack = p.stack[:len(p.stack)-1]
	}
}

// registerNodeMembership records that a node/subgraph belongs to the current
// open subgraph.
func (p *parser) registerMembership(id string) {
	par := p.curParent()
	if par == "" {
		return
	}
	if sg := p.g.Subgraphs[par]; sg != nil {
		sg.Children = appendUnique(sg.Children, id)
	}
	if n := p.g.Nodes[id]; n != nil && n.Parent == "" {
		n.Parent = par
	}
}

func appendUnique(s []string, v string) []string {
	for _, x := range s {
		if x == v {
			return s
		}
	}
	return append(s, v)
}
