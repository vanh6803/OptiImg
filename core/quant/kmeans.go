package quant

func nearest(pal []col, c col) (int, float32) {
	bi, bd := 0, dist(pal[0], c)
	for i := 1; i < len(pal); i++ {
		if d := dist(pal[i], c); d < bd {
			bi, bd = i, d
		}
	}
	return bi, bd
}

// kmeans refines pal over the weighted bins: at most 5 rounds, stopping when
// the total error improves by less than 0.5%.
func kmeans(bins []bin, pal []col) []col {
	prev := float64(-1)
	for round := 0; round < 5; round++ {
		sum := make([][4]float64, len(pal))
		cnt := make([]float64, len(pal))
		var total float64
		for _, b := range bins {
			i, d := nearest(pal, b.c)
			total += float64(d) * float64(b.n)
			for c := 0; c < 4; c++ {
				sum[i][c] += float64(b.c[c]) * float64(b.n)
			}
			cnt[i] += float64(b.n)
		}
		for i := range pal {
			if cnt[i] > 0 {
				pal[i] = col{float32(sum[i][0] / cnt[i]), float32(sum[i][1] / cnt[i]), float32(sum[i][2] / cnt[i]), float32(sum[i][3] / cnt[i])}
			}
		}
		if prev >= 0 && (prev-total) < 0.005*prev {
			break
		}
		prev = total
	}
	return pal
}
