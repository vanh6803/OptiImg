package quant

import "sort"

func sortBins(b []bin, less func(x, y bin) bool) {
	sort.Slice(b, func(i, j int) bool { return less(b[i], b[j]) })
}

var chanW = [4]float32{wR, wG, wB, wA}

type box struct {
	bins  []bin
	n     int
	ch    int     // channel with the largest weighted spread
	score float64 // spread × pixel count
}

func newBox(bins []bin) box {
	b := box{bins: bins}
	var lo, hi col
	for i, x := range bins {
		b.n += x.n
		for c := 0; c < 4; c++ {
			if i == 0 || x.c[c] < lo[c] {
				lo[c] = x.c[c]
			}
			if i == 0 || x.c[c] > hi[c] {
				hi[c] = x.c[c]
			}
		}
	}
	var best float32
	for c := 0; c < 4; c++ {
		r := hi[c] - lo[c]
		if s := chanW[c] * r * r; s > best {
			best, b.ch = s, c
		}
	}
	if len(bins) > 1 {
		b.score = float64(best) * float64(b.n)
	}
	return b
}

func (b box) mean() col {
	var s [4]float64
	for _, x := range b.bins {
		for c := 0; c < 4; c++ {
			s[c] += float64(x.c[c]) * float64(x.n)
		}
	}
	f := float64(b.n)
	return col{float32(s[0] / f), float32(s[1] / f), float32(s[2] / f), float32(s[3] / f)}
}

// medianCut returns up to target representative colours.
func medianCut(bins []bin, target int) []col {
	if target < 1 {
		target = 1
	}
	boxes := []box{newBox(append([]bin(nil), bins...))}
	for len(boxes) < target {
		bi, best := -1, 0.0
		for i, b := range boxes {
			if b.score > best {
				bi, best = i, b.score
			}
		}
		if bi < 0 {
			break
		}
		b := boxes[bi]
		ch := b.ch
		sortBins(b.bins, func(x, y bin) bool { return x.c[ch] < y.c[ch] })
		half, acc, cut := b.n/2, 0, 1
		for i, x := range b.bins {
			acc += x.n
			if acc >= half {
				cut = i + 1
				break
			}
		}
		if cut >= len(b.bins) {
			cut = len(b.bins) - 1
		}
		boxes[bi] = newBox(b.bins[:cut])
		boxes = append(boxes, newBox(b.bins[cut:]))
	}
	out := make([]col, len(boxes))
	for i, b := range boxes {
		out[i] = b.mean()
	}
	return out
}
