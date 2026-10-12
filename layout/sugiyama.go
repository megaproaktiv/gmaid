package layout

import (
	"sort"
)

// vertex is an internal layout node (either a real node or a dummy created to
// route a long edge). Sizes are intrinsic (already measured).
type vertex struct {
	id    string
	w, h  float64 // intrinsic size (unrotated, in flow-agnostic terms)
	dummy bool
	rank  int
	order int     // position within its rank
	x, y  float64 // final top-left, in the group's local coordinate space
}

// link is an internal edge between vertices after cycle breaking.
type link struct {
	from, to string
	orig     *edgeRef // nil for dummy chain segments
	reversed bool
}

// edgeRef carries the original edge plus its routed chain of vertex ids.
type edgeRef struct {
	key   int
	chain []string // real->dummy...->real vertex ids in flow order
}

// group is a self-contained layout problem: a set of vertices and links laid
// out along a flow axis. Direction maps to axis orientation.
type group struct {
	cfg         Config
	flow        string // "TB", "LR", "BT", "RL"
	verts       map[string]*vertex
	order       []string // deterministic vertex id order
	links       []link
	layersCache [][]string
	// labelExtent is extra rank-axis spacing reserved so edge labels fit
	// between ranks (mermaid widens labeled gaps). Set by the engine.
	labelExtent float64
}

// isHorizontal reports whether the flow axis is horizontal (LR/RL), meaning
// ranks advance along X and ordering spreads along Y.
func (g *group) isHorizontal() bool { return g.flow == "LR" || g.flow == "RL" }

// solve runs the full pipeline and assigns x,y (top-left) to every vertex in
// the group's local coordinate space. It returns the chains for edge routing.
func (g *group) solve() map[int][]string {
	g.breakCycles()
	g.assignRanks()
	chains := g.insertDummies()
	g.ordering()
	g.assignCoords()
	return chains
}

// --- 1. cycle breaking (DFS, reverse back-edges) ---

func (g *group) breakCycles() {
	const (
		white = 0
		gray  = 1
		black = 2
	)
	color := map[string]int{}
	adj := map[string][]int{} // vertex id -> indices into g.links
	for i := range g.links {
		adj[g.links[i].from] = append(adj[g.links[i].from], i)
	}
	var dfs func(u string)
	dfs = func(u string) {
		color[u] = gray
		for _, li := range adj[u] {
			l := &g.links[li]
			if l.from != u {
				continue
			}
			switch color[l.to] {
			case gray:
				// back edge -> reverse it
				l.from, l.to = l.to, l.from
				l.reversed = true
			case white:
				dfs(l.to)
			}
		}
		color[u] = black
	}
	for _, id := range g.order {
		if color[id] == white {
			dfs(id)
		}
	}
}

// --- 2. rank assignment (longest path from sources) ---

func (g *group) assignRanks() {
	indeg := map[string]int{}
	adj := map[string][]int{}
	for _, id := range g.order {
		indeg[id] = 0
	}
	for i := range g.links {
		l := g.links[i]
		if l.from == l.to {
			continue
		}
		adj[l.from] = append(adj[l.from], i)
		indeg[l.to]++
	}
	// Kahn topological order, assigning rank = max(pred.rank + minlen).
	queue := []string{}
	for _, id := range g.order {
		if indeg[id] == 0 {
			queue = append(queue, id)
			g.verts[id].rank = 0
		}
	}
	processed := 0
	for len(queue) > 0 {
		u := queue[0]
		queue = queue[1:]
		processed++
		for _, li := range adj[u] {
			l := g.links[li]
			ml := 1
			if l.orig != nil {
				// min length applied later via dummies; keep 1 here
			}
			if r := g.verts[u].rank + ml; r > g.verts[l.to].rank {
				g.verts[l.to].rank = r
			}
			indeg[l.to]--
			if indeg[l.to] == 0 {
				queue = append(queue, l.to)
			}
		}
	}
	// Any leftover (shouldn't happen after cycle breaking) gets rank by scan.
	_ = processed

	g.stackIsolated()
}

// stackIsolated handles vertices that have no edges within this group (common
// for subgraph members that only connect to nodes outside the subgraph). A
// pure layered layout would place them all at rank 0 and spread them along the
// cross axis, but mermaid stacks such independent nodes along the flow (rank)
// axis — e.g. a column for a TB subgraph. We reproduce that by giving each
// internally-isolated vertex its own consecutive rank, interleaved so they do
// not collide with connected components.
func (g *group) stackIsolated() {
	deg := map[string]int{}
	for _, l := range g.links {
		if l.from == l.to {
			continue
		}
		deg[l.from]++
		deg[l.to]++
	}

	// Collect isolated vertices in deterministic order.
	var isolated []string
	connected := false
	for _, id := range g.order {
		if g.verts[id].dummy {
			continue
		}
		if deg[id] == 0 {
			isolated = append(isolated, id)
		} else {
			connected = true
		}
	}
	if len(isolated) == 0 {
		return
	}

	if !connected {
		// Whole group is isolated nodes: simply stack them 0,1,2,...
		for i, id := range isolated {
			g.verts[id].rank = i
		}
		return
	}

	// Mixed: place isolated nodes after the connected ranks, stacked, so they
	// form their own column/row without overlapping connected content.
	maxRank := 0
	for _, id := range g.order {
		if !g.verts[id].dummy && deg[id] > 0 && g.verts[id].rank > maxRank {
			maxRank = g.verts[id].rank
		}
	}
	for i, id := range isolated {
		g.verts[id].rank = i // stack independent nodes down the flow axis
	}
	_ = maxRank
}

// --- 3. dummy insertion for edges spanning more than one rank ---

func (g *group) insertDummies() map[int][]string {
	chains := map[int][]string{}
	var newLinks []link
	dummyCount := 0
	for i := range g.links {
		l := g.links[i]
		if l.orig == nil {
			newLinks = append(newLinks, l)
			continue
		}
		rf := g.verts[l.from].rank
		rt := g.verts[l.to].rank
		if rt < rf {
			rf, rt = rt, rf
		}
		if rt-rf <= 1 {
			newLinks = append(newLinks, l)
			chains[l.orig.key] = orientChain([]string{l.from, l.to}, l.reversed)
			continue
		}
		// create dummy vertices for intermediate ranks
		chain := []string{l.from}
		prev := l.from
		for r := rf + 1; r < rt; r++ {
			did := dummyID(dummyCount)
			dummyCount++
			dv := &vertex{id: did, dummy: true, rank: r, w: 1, h: 1}
			g.verts[did] = dv
			g.order = append(g.order, did)
			newLinks = append(newLinks, link{from: prev, to: did})
			chain = append(chain, did)
			prev = did
		}
		newLinks = append(newLinks, link{from: prev, to: l.to})
		chain = append(chain, l.to)
		chains[l.orig.key] = orientChain(chain, l.reversed)
	}
	g.links = newLinks
	return chains
}

func orientChain(chain []string, reversed bool) []string {
	if !reversed {
		return chain
	}
	out := make([]string, len(chain))
	for i := range chain {
		out[i] = chain[len(chain)-1-i]
	}
	return out
}

func dummyID(n int) string {
	return "\x00dummy" + itoa(n)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

// --- 4. ordering within ranks (barycenter + transpose) ---

func (g *group) ranks() [][]string {
	maxRank := 0
	for _, v := range g.verts {
		if v.rank > maxRank {
			maxRank = v.rank
		}
	}
	layers := make([][]string, maxRank+1)
	// stable initial order from g.order
	for _, id := range g.order {
		v := g.verts[id]
		layers[v.rank] = append(layers[v.rank], id)
	}
	for r := range layers {
		for i, id := range layers[r] {
			g.verts[id].order = i
		}
	}
	return layers
}

func (g *group) ordering() {
	layers := g.ranks()
	// Build adjacency for barycenter.
	up := map[string][]string{}   // neighbors in previous rank
	down := map[string][]string{} // neighbors in next rank
	for _, l := range g.links {
		rf, rt := g.verts[l.from].rank, g.verts[l.to].rank
		if rf == rt {
			continue
		}
		if rf < rt {
			down[l.from] = append(down[l.from], l.to)
			up[l.to] = append(up[l.to], l.from)
		} else {
			down[l.to] = append(down[l.to], l.from)
			up[l.from] = append(up[l.from], l.to)
		}
	}

	setOrder := func() {
		for r := range layers {
			for i, id := range layers[r] {
				g.verts[id].order = i
			}
		}
	}
	setOrder()

	bary := func(id string, nbrs map[string][]string) float64 {
		ns := nbrs[id]
		if len(ns) == 0 {
			return float64(g.verts[id].order)
		}
		sum := 0.0
		for _, n := range ns {
			sum += float64(g.verts[n].order)
		}
		return sum / float64(len(ns))
	}

	for iter := 0; iter < 8; iter++ {
		// downward sweep uses up-neighbors
		for r := 1; r < len(layers); r++ {
			sortLayerBy(layers[r], g, up, bary)
			reindex(layers[r], g)
		}
		// upward sweep uses down-neighbors
		for r := len(layers) - 2; r >= 0; r-- {
			sortLayerBy(layers[r], g, down, bary)
			reindex(layers[r], g)
		}
	}
	setOrder()
	g.layersCache = layers
}

func sortLayerBy(layer []string, g *group, nbrs map[string][]string, bary func(string, map[string][]string) float64) {
	keys := make(map[string]float64, len(layer))
	for _, id := range layer {
		keys[id] = bary(id, nbrs)
	}
	sort.SliceStable(layer, func(i, j int) bool {
		return keys[layer[i]] < keys[layer[j]]
	})
}

func reindex(layer []string, g *group) {
	for i, id := range layer {
		g.verts[id].order = i
	}
}
