package layout

// assignCoords places every vertex (top-left x,y) in the group's local space.
//
// We think in terms of two axes:
//   - rank axis:  the direction the flow progresses (ranks are stacked here)
//   - cross axis: perpendicular, nodes within a rank are spread here
//
// For TB/BT the rank axis is vertical (Y) and cross axis horizontal (X).
// For LR/RL the rank axis is horizontal (X) and cross axis vertical (Y).
// BT and RL simply reverse the rank-axis order.
func (g *group) assignCoords() {
	layers := g.layersCache
	if layers == nil {
		layers = g.ranks()
	}
	horizontal := g.isHorizontal()

	// crossSize returns the vertex extent along the cross axis.
	crossSize := func(v *vertex) float64 {
		if horizontal {
			return v.h
		}
		return v.w
	}
	// rankSize returns the vertex extent along the rank axis.
	rankSize := func(v *vertex) float64 {
		if horizontal {
			return v.w
		}
		return v.h
	}

	// 1. Rank-axis coordinate: stack layers separated by RankSep, each layer
	//    positioned at the running offset plus half the tallest vertex.
	//    Horizontal flows (LR/RL) use a larger rank gap: mermaid renders such
	//    chains very flat and wide, whereas vertical (TB) flows stay compact.
	rankSep := g.cfg.RankSep
	if horizontal {
		rankSep *= 1.4
	}
	rankPos := make([]float64, len(layers)) // center along rank axis per layer
	off := 0.0
	for r, layer := range layers {
		maxRank := 0.0
		for _, id := range layer {
			if s := rankSize(g.verts[id]); s > maxRank {
				maxRank = s
			}
		}
		rankPos[r] = off + maxRank/2
		off += maxRank + rankSep + g.labelExtent
	}

	// 2. Cross-axis coordinate: within each layer, place vertices left to
	//    right separated by NodeSep. Then align via barycenter of neighbors
	//    across a few passes to straighten edges.
	crossCenter := map[string]float64{}
	place := func(layer []string) {
		pos := 0.0
		for _, id := range layer {
			cs := crossSize(g.verts[id])
			crossCenter[id] = pos + cs/2
			pos += cs + g.cfg.NodeSep
		}
	}
	for _, layer := range layers {
		place(layer)
	}

	// neighbor maps for alignment
	up := map[string][]string{}
	down := map[string][]string{}
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

	// weightedTarget returns the barycenter (mean) of a vertex's combined up-
	// and down-neighbor cross positions. Balancing parents and children at
	// once keeps the layout centered.
	weightedTarget := func(id string) (float64, bool) {
		vals := make([]float64, 0, len(up[id])+len(down[id]))
		for _, u := range up[id] {
			vals = append(vals, crossCenter[u])
		}
		for _, d := range down[id] {
			vals = append(vals, crossCenter[d])
		}
		if len(vals) == 0 {
			return 0, false
		}
		s := 0.0
		for _, v := range vals {
			s += v
		}
		return s / float64(len(vals)), true
	}

	// alignment passes: pull each vertex toward its combined neighbor
	// barycenter, then resolve overlaps within the layer to keep separation
	// and order. Sweeping both directions avoids systematic bias.
	for iter := 0; iter < 24; iter++ {
		downward := iter%2 == 0
		rng := make([]int, len(layers))
		for i := range rng {
			if downward {
				rng[i] = i
			} else {
				rng[i] = len(layers) - 1 - i
			}
		}
		for _, r := range rng {
			layer := layers[r]
			for _, id := range layer {
				if want, ok := weightedTarget(id); ok {
					crossCenter[id] = want
				}
			}
			resolveOverlaps(layer, crossCenter, crossSize, g.verts, g.cfg.NodeSep)
		}
	}

	// Chain straightening: a node joined to a neighbor by a 1-to-1 link (the
	// node's only predecessor and that predecessor's only successor, or the
	// symmetric successor case) should share the neighbor's cross coordinate
	// so linear chains render as straight lines instead of staircases. We snap
	// toward the neighbor, then re-resolve overlaps so order is preserved.
	outDeg := map[string]int{}
	inDeg := map[string]int{}
	for _, l := range g.links {
		rf, rt := g.verts[l.from].rank, g.verts[l.to].rank
		if rf == rt {
			continue
		}
		outDeg[l.from]++
		inDeg[l.to]++
	}
	for pass := 0; pass < 6; pass++ {
		for r := 1; r < len(layers); r++ {
			for _, id := range layers[r] {
				ups := up[id]
				if len(ups) == 1 {
					p := ups[0]
					// align only when this is the predecessor's sole successor
					if outDeg[p] == 1 && inDeg[id] == 1 {
						crossCenter[id] = crossCenter[p]
					}
				}
			}
			resolveOverlaps(layers[r], crossCenter, crossSize, g.verts, g.cfg.NodeSep)
		}
		for r := len(layers) - 2; r >= 0; r-- {
			for _, id := range layers[r] {
				downs := down[id]
				if len(downs) == 1 {
					c := downs[0]
					if inDeg[c] == 1 && outDeg[id] == 1 {
						crossCenter[id] = crossCenter[c]
					}
				}
			}
			resolveOverlaps(layers[r], crossCenter, crossSize, g.verts, g.cfg.NodeSep)
		}
	}

	minCross := 1e18
	for _, layer := range layers {
		for _, id := range layer {
			v := g.verts[id]
			c := crossCenter[id] - crossSize(v)/2
			if c < minCross {
				minCross = c
			}
		}
	}
	if minCross == 1e18 {
		minCross = 0
	}

	maxRankIdx := len(layers) - 1
	for r, layer := range layers {
		rp := rankPos[r]
		if g.flow == "BT" || g.flow == "RL" {
			rp = rankPos[maxRankIdx] - (rp - rankPos[0])
			// reflect: use mirrored center so later ranks go toward origin
			rp = mirrorRank(rankPos, r)
		}
		for _, id := range layer {
			v := g.verts[id]
			cc := crossCenter[id] - minCross
			if horizontal {
				// rank axis = X, cross axis = Y
				v.x = rp - v.w/2
				v.y = cc - v.h/2
			} else {
				// rank axis = Y, cross axis = X
				v.x = cc - v.w/2
				v.y = rp - v.h/2
			}
		}
	}
}

// mirrorRank reflects rank positions for BT/RL so rank 0 sits at the far end.
func mirrorRank(rankPos []float64, r int) float64 {
	last := len(rankPos) - 1
	return rankPos[last] - (rankPos[r] - rankPos[0])
}

// resolveOverlaps enforces minimum separation along the cross axis while
// preserving the given order of the layer. It nudges vertices rightward then
// recenters leftward to avoid a systematic drift.
func resolveOverlaps(layer []string, center map[string]float64, size func(*vertex) float64, verts map[string]*vertex, sep float64) {
	// forward pass: ensure each vertex is at least sep after the previous
	for i := 1; i < len(layer); i++ {
		prev := verts[layer[i-1]]
		cur := verts[layer[i]]
		minC := center[layer[i-1]] + size(prev)/2 + sep + size(cur)/2
		if center[layer[i]] < minC {
			center[layer[i]] = minC
		}
	}
	// backward pass: pull back if there is slack (keeps clusters compact)
	for i := len(layer) - 2; i >= 0; i-- {
		next := verts[layer[i+1]]
		cur := verts[layer[i]]
		maxC := center[layer[i+1]] - size(next)/2 - sep - size(cur)/2
		if center[layer[i]] > maxC {
			center[layer[i]] = maxC
		}
	}
}
