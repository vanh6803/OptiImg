package jpegenc

import (
	"errors"
	"sort"
)

// ErrNotOptimizable is returned by OptimizeHuffman for JPEGs it does not
// handle (progressive, restart intervals, several scans, 12-bit, …).
var ErrNotOptimizable = errors.New("jpegenc: JPEG layout not supported by the Huffman optimiser")

type jcomp struct{ id, h, v, td, ta int }

type huffTable struct {
	bits [17]int // bits[i] = number of codes of length i
	vals []uint8
	// canonical codes, filled by finish
	code [256]uint16
	size [256]uint8
	// decoding
	mincode, maxcode [18]int
	valptr           [17]int
}

func (t *huffTable) finish() {
	code, k := 0, 0
	for l := 1; l <= 16; l++ {
		t.valptr[l] = k
		t.mincode[l] = code
		for i := 0; i < t.bits[l]; i++ {
			s := t.vals[k]
			t.code[s], t.size[s] = uint16(code), uint8(l)
			code++
			k++
		}
		t.maxcode[l] = code - 1
		if t.bits[l] == 0 {
			t.maxcode[l] = -1
		}
		code <<= 1
	}
}

// OptimizeHuffman losslessly rewrites a baseline sequential JPEG with
// Huffman tables built from the image's actual symbol statistics. It works
// in two streaming passes over the entropy data, so memory stays at a few
// times the file size. It returns the smaller of the input and the result.
func OptimizeHuffman(in []byte) ([]byte, error) {
	if len(in) < 4 || in[0] != 0xFF || in[1] != 0xD8 {
		return nil, ErrNotOptimizable
	}
	var comps []jcomp // SOF components
	var width, height int
	tables := map[int]*huffTable{} // key class<<4|id
	var head []byte                // segments before SOS, minus DHT
	sosHdr := []byte(nil)
	var scanComps []jcomp
	p := 2
	head = append(head, 0xFF, 0xD8)
	var entStart int
loop:
	for p+4 <= len(in) {
		if in[p] != 0xFF {
			return nil, ErrNotOptimizable
		}
		m := in[p+1]
		n := int(in[p+2])<<8 | int(in[p+3])
		if n < 2 || p+2+n > len(in) {
			return nil, ErrNotOptimizable
		}
		seg := in[p : p+2+n]
		body := seg[4:]
		switch {
		case m == 0xC0:
			if len(body) < 6 || body[0] != 8 {
				return nil, ErrNotOptimizable
			}
			height = int(body[1])<<8 | int(body[2])
			width = int(body[3])<<8 | int(body[4])
			nc := int(body[5])
			if nc != 1 && nc != 3 || len(body) < 6+3*nc {
				return nil, ErrNotOptimizable
			}
			for i := 0; i < nc; i++ {
				b := body[6+3*i:]
				comps = append(comps, jcomp{id: int(b[0]), h: int(b[1] >> 4), v: int(b[1] & 15)})
			}
			head = append(head, seg...)
		case m == 0xC4:
			for len(body) >= 17 {
				tc, th := int(body[0]>>4), int(body[0]&15)
				t := &huffTable{}
				total := 0
				for l := 1; l <= 16; l++ {
					t.bits[l] = int(body[l])
					total += t.bits[l]
				}
				if len(body) < 17+total || total > 256 {
					return nil, ErrNotOptimizable
				}
				t.vals = append([]uint8(nil), body[17:17+total]...)
				t.finish()
				tables[tc<<4|th] = t
				body = body[17+total:]
			}
		case m == 0xDD:
			if len(body) >= 2 && (body[0]|body[1]) != 0 {
				return nil, ErrNotOptimizable // restart intervals
			}
			head = append(head, seg...)
		case m == 0xC1 || (m >= 0xC2 && m <= 0xCF && m != 0xC4 && m != 0xC8 && m != 0xCC):
			return nil, ErrNotOptimizable // not baseline
		case m == 0xDA:
			if comps == nil || len(body) < 1 {
				return nil, ErrNotOptimizable
			}
			ns := int(body[0])
			if ns != len(comps) || len(body) < 1+2*ns+3 {
				return nil, ErrNotOptimizable
			}
			for i := 0; i < ns; i++ {
				cid, tt := int(body[1+2*i]), body[2+2*i]
				found := false
				for _, c := range comps {
					if c.id == cid {
						c.td, c.ta = int(tt>>4), int(tt&15)
						scanComps = append(scanComps, c)
						found = true
					}
				}
				if !found {
					return nil, ErrNotOptimizable
				}
			}
			sosHdr = seg
			entStart = p + 2 + n
			break loop
		default:
			head = append(head, seg...)
		}
		p += 2 + n
	}
	if sosHdr == nil || width == 0 || height == 0 {
		return nil, ErrNotOptimizable
	}
	// find end of entropy data
	entEnd := entStart
	for entEnd+1 < len(in) {
		if in[entEnd] == 0xFF && in[entEnd+1] != 0x00 && !(in[entEnd+1] >= 0xD0 && in[entEnd+1] <= 0xD7) {
			break
		}
		entEnd++
	}
	if entEnd+1 >= len(in) {
		return nil, ErrNotOptimizable
	}
	if in[entEnd+1] != 0xD9 {
		return nil, ErrNotOptimizable // another scan or unexpected marker
	}
	data, tail := in[entStart:entEnd], in[entEnd:]

	// MCU layout
	hmax, vmax := 1, 1
	for _, c := range comps {
		if c.h > hmax {
			hmax = c.h
		}
		if c.v > vmax {
			vmax = c.v
		}
	}
	var mcus int
	var blocks []jcomp // per-MCU block sequence (component repeated h*v times)
	if len(scanComps) == 1 {
		c := scanComps[0]
		cw, ch := (width*c.h+hmax-1)/hmax, (height*c.v+vmax-1)/vmax
		mcus = ((cw + 7) / 8) * ((ch + 7) / 8)
		blocks = []jcomp{c}
	} else {
		mcus = ((width + 8*hmax - 1) / (8 * hmax)) * ((height + 8*vmax - 1) / (8 * vmax))
		for _, c := range scanComps {
			for i := 0; i < c.h*c.v; i++ {
				blocks = append(blocks, c)
			}
		}
	}
	for _, c := range blocks {
		if tables[0<<4|c.td] == nil || tables[1<<4|c.ta] == nil {
			return nil, ErrNotOptimizable
		}
	}

	// pass 1: symbol frequencies
	var freq [2][4][257]int
	if err := walk(data, mcus, blocks, tables, func(class, id int, sym uint8, nb uint, bits uint32) {
		freq[class][id&3][sym]++
	}); err != nil {
		return nil, err
	}
	newTables := map[int]*huffTable{}
	for class := 0; class < 2; class++ {
		for id := 0; id < 4; id++ {
			used := false
			for _, f := range freq[class][id][:256] {
				if f > 0 {
					used = true
					break
				}
			}
			if used {
				newTables[class<<4|id] = optimalTable(freq[class][id])
			}
		}
	}
	// pass 2: re-encode
	w := &bitWriter{out: make([]byte, 0, len(data))}
	if err := walk(data, mcus, blocks, tables, func(class, id int, sym uint8, nb uint, bits uint32) {
		t := newTables[class<<4|id&3]
		w.write(uint32(t.code[sym]), uint(t.size[sym]))
		if nb > 0 {
			w.write(bits, nb)
		}
	}); err != nil {
		return nil, err
	}
	w.flush()

	// assemble
	out := append([]byte(nil), head...)
	keys := make([]int, 0, len(newTables))
	for k := range newTables {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	for _, k := range keys {
		t := newTables[k]
		n := 2 + 1 + 16 + len(t.vals)
		out = append(out, 0xFF, 0xC4, byte(n>>8), byte(n), byte(k>>4<<4|k&15))
		for l := 1; l <= 16; l++ {
			out = append(out, byte(t.bits[l]))
		}
		out = append(out, t.vals...)
	}
	out = append(out, sosHdr...)
	out = append(out, w.out...)
	out = append(out, tail...)
	if len(out) >= len(in) {
		return in, nil
	}
	return out, nil
}

// bitReader reads MSB-first bits from entropy-coded data with 0xFF00 unstuffing.
type bitReader struct {
	data []byte
	pos  int
	acc  uint32
	n    uint
}

func (r *bitReader) bit() (uint32, error) {
	if r.n == 0 {
		if r.pos >= len(r.data) {
			return 0, ErrNotOptimizable
		}
		b := r.data[r.pos]
		r.pos++
		if b == 0xFF {
			if r.pos >= len(r.data) || r.data[r.pos] != 0x00 {
				return 0, ErrNotOptimizable
			}
			r.pos++
		}
		r.acc, r.n = uint32(b), 8
	}
	r.n--
	return (r.acc >> r.n) & 1, nil
}

func (r *bitReader) bits(n uint) (uint32, error) {
	var v uint32
	for i := uint(0); i < n; i++ {
		b, err := r.bit()
		if err != nil {
			return 0, err
		}
		v = v<<1 | b
	}
	return v, nil
}

func (r *bitReader) symbol(t *huffTable) (uint8, error) {
	code := 0
	for l := 1; l <= 16; l++ {
		b, err := r.bit()
		if err != nil {
			return 0, err
		}
		code = code<<1 | int(b)
		if t.maxcode[l] >= 0 && code <= t.maxcode[l] && code >= t.mincode[l] {
			idx := t.valptr[l] + code - t.mincode[l]
			if idx >= len(t.vals) {
				return 0, ErrNotOptimizable
			}
			return t.vals[idx], nil
		}
	}
	return 0, ErrNotOptimizable
}

// walk decodes the entropy-coded data, calling emit for every Huffman symbol
// together with its raw extra bits.
func walk(data []byte, mcus int, blocks []jcomp, tables map[int]*huffTable, emit func(class, id int, sym uint8, nb uint, bits uint32)) error {
	r := &bitReader{data: data}
	for m := 0; m < mcus; m++ {
		for _, c := range blocks {
			dc, ac := tables[0<<4|c.td], tables[1<<4|c.ta]
			s, err := r.symbol(dc)
			if err != nil {
				return err
			}
			if s > 11 {
				return ErrNotOptimizable
			}
			v, err := r.bits(uint(s))
			if err != nil {
				return err
			}
			emit(0, c.td, s, uint(s), v)
			for k := 1; k < 64; {
				rs, err := r.symbol(ac)
				if err != nil {
					return err
				}
				run, size := int(rs>>4), uint(rs&15)
				if size == 0 {
					emit(1, c.ta, rs, 0, 0)
					if run == 15 {
						k += 16
						continue
					}
					break // EOB
				}
				v, err := r.bits(size)
				if err != nil {
					return err
				}
				emit(1, c.ta, rs, size, v)
				k += run + 1
			}
		}
	}
	return nil
}

// optimalTable builds a length-limited (16-bit) Huffman table from symbol
// frequencies using the procedure of ITU T.81 Annex K.2.
func optimalTable(freq [257]int) *huffTable {
	f := freq
	f[256] = 1 // reserve one code point so no code is all ones
	var codesize [257]int
	var others [257]int
	for i := range others {
		others[i] = -1
	}
	for {
		c1, c2 := -1, -1
		v := int(^uint(0) >> 1)
		for i := 0; i <= 256; i++ {
			if f[i] != 0 && f[i] <= v {
				v, c1 = f[i], i
			}
		}
		v = int(^uint(0) >> 1)
		for i := 0; i <= 256; i++ {
			if f[i] != 0 && f[i] <= v && i != c1 {
				v, c2 = f[i], i
			}
		}
		if c2 < 0 {
			break
		}
		f[c1] += f[c2]
		f[c2] = 0
		codesize[c1]++
		for others[c1] >= 0 {
			c1 = others[c1]
			codesize[c1]++
		}
		others[c1] = c2
		codesize[c2]++
		for others[c2] >= 0 {
			c2 = others[c2]
			codesize[c2]++
		}
	}
	var bits [34]int
	for i := 0; i <= 256; i++ {
		if codesize[i] > 0 {
			bits[codesize[i]]++
		}
	}
	for i := 32; i > 16; i-- {
		for bits[i] > 0 {
			j := i - 2
			for bits[j] == 0 {
				j--
			}
			bits[i] -= 2
			bits[i-1]++
			bits[j+1] += 2
			bits[j]--
		}
	}
	i := 16
	for bits[i] == 0 {
		i--
	}
	bits[i]-- // drop the reserved symbol
	t := &huffTable{}
	for l := 1; l <= 16; l++ {
		t.bits[l] = bits[l]
	}
	for l := 1; l <= 32; l++ {
		for s := 0; s < 256; s++ {
			if codesize[s] == l {
				t.vals = append(t.vals, uint8(s))
			}
		}
	}
	t.finish()
	return t
}

type bitWriter struct {
	out []byte
	acc uint32
	n   uint
}

func (w *bitWriter) write(v uint32, n uint) {
	for n > 0 {
		take := 8 - w.n
		if take > n {
			take = n
		}
		w.acc = w.acc<<take | (v>>(n-take))&(1<<take-1)
		w.n += take
		n -= take
		if w.n == 8 {
			b := byte(w.acc)
			w.out = append(w.out, b)
			if b == 0xFF {
				w.out = append(w.out, 0x00)
			}
			w.acc, w.n = 0, 0
		}
	}
}

// flush pads the last byte with 1 bits.
func (w *bitWriter) flush() {
	if w.n > 0 {
		w.write(0xFF, 8-w.n)
	}
}
