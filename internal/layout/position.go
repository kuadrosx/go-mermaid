package layout

import "github.com/zkrebbekx/go-mermaid/internal/domain"

// positionLG assigns coordinates in rank space: pc is the center along the
// rank (primary) axis, cross is the coordinate along the order (cross) axis.
// The cross axis is aligned with a barycenter heuristic so parents sit over
// their children and edges run as straight as possible. Returns the total
// primary extent. Mapping to screen x/y (and direction flips) happens later.
func positionLG(lg *lgraph, dir domain.Direction, opts Options) float64 {
	vertical := dir == domain.TopBottom || dir == domain.BottomTop

	// Primary axis: stack ranks, centering each node in its rank band.
	var primary float64
	for _, layer := range lg.layers {
		var band float64
		for _, ln := range layer {
			if s := primarySize(ln, vertical); s > band {
				band = s
			}
			// In LR the loop label sits below the node, centred on it, and
			// may be wider than the node itself.
			if !vertical && ln.loop.W > band {
				band = ln.loop.W
			}
		}
		for _, ln := range layer {
			ln.pc = primary + band/2
		}
		primary += band + opts.RankSep
	}

	// Cross axis: initial left pack, then barycenter sweeps.
	for _, layer := range lg.layers {
		var c float64
		for _, ln := range layer {
			ln.cross = c + crossSize(ln, vertical)/2
			c = ln.cross + crossAfter(ln, vertical) + opts.NodeSep
		}
	}
	for iter := 0; iter < 8; iter++ {
		if iter%2 == 0 {
			for r := 1; r < len(lg.layers); r++ {
				alignRank(lg.layers[r], lg.ups, vertical, opts)
			}
		} else {
			for r := len(lg.layers) - 2; r >= 0; r-- {
				alignRank(lg.layers[r], lg.downs, vertical, opts)
			}
		}
	}

	normalizeCross(lg, vertical)
	if primary > 0 {
		primary -= opts.RankSep // drop trailing gap
	}
	return primary
}

// alignRank pulls each node toward the average cross of its neighbors, then
// resolves overlaps left to right while preserving the rank's order.
func alignRank(layer []*lnode, adj map[*lnode][]*lnode, vertical bool, opts Options) {
	for _, ln := range layer {
		ns := adj[ln]
		if len(ns) == 0 {
			continue
		}
		var sum float64
		for _, nb := range ns {
			sum += nb.cross
		}
		ln.cross = sum / float64(len(ns))
	}
	for i := 1; i < len(layer); i++ {
		prev, cur := layer[i-1], layer[i]
		minC := prev.cross + crossAfter(prev, vertical) + opts.NodeSep + crossSize(cur, vertical)/2
		if cur.cross < minC {
			cur.cross = minC
		}
	}
}

// normalizeCross shifts all nodes so the left-most box edge is at 0.
func normalizeCross(lg *lgraph, vertical bool) {
	minLeft := 0.0
	first := true
	for _, layer := range lg.layers {
		for _, ln := range layer {
			left := ln.cross - crossSize(ln, vertical)/2
			if first || left < minLeft {
				minLeft, first = left, false
			}
		}
	}
	if minLeft == 0 {
		return
	}
	for _, layer := range lg.layers {
		for _, ln := range layer {
			ln.cross -= minLeft
		}
	}
}

func crossSize(ln *lnode, vertical bool) float64 {
	if vertical {
		return ln.w
	}
	return ln.h
}

// crossAfter is the room a node needs past its cross center: half its box,
// plus the loop and label of a labelled self-loop, which sit on that side.
func crossAfter(ln *lnode, vertical bool) float64 {
	after := crossSize(ln, vertical) / 2
	if ln.loop.W > 0 {
		after += loopOut + labelGap
		if vertical {
			after += ln.loop.W
		} else {
			after += ln.loop.H
		}
	}
	return after
}

func primarySize(ln *lnode, vertical bool) float64 {
	if vertical {
		return ln.h
	}
	return ln.w
}
