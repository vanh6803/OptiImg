package core

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"hash/crc32"
	"io"
)

// extractICC returns the raw ICC profile embedded in a PNG (iCCP) or JPEG
// (APP2 "ICC_PROFILE" chunks), or nil.
func extractICC(b []byte, f Format) []byte {
	switch f {
	case FormatPNG:
		if !bytes.HasPrefix(b, pngMagic) {
			return nil
		}
		for p := len(pngMagic); p+8 <= len(b); {
			n := int(binary.BigEndian.Uint32(b[p:]))
			typ := string(b[p+4 : p+8])
			if p+12+n > len(b) {
				return nil
			}
			switch typ {
			case "iCCP":
				body := b[p+8 : p+8+n]
				i := bytes.IndexByte(body, 0)
				if i < 0 || i+2 > len(body) {
					return nil
				}
				zr, err := zlib.NewReader(bytes.NewReader(body[i+2:]))
				if err != nil {
					return nil
				}
				prof, err := io.ReadAll(io.LimitReader(zr, 16<<20))
				if err != nil || len(prof) < 128 {
					return nil
				}
				return prof
			case "IDAT", "IEND":
				return nil
			}
			p += 12 + n
		}
	case FormatJPEG:
		type part struct {
			seq  int
			data []byte
		}
		var parts []part
		for p := 2; p+4 <= len(b) && b[p] == 0xFF; {
			m := b[p+1]
			if m == 0xDA || m == 0xD9 {
				break
			}
			n := int(binary.BigEndian.Uint16(b[p+2:]))
			if n < 2 || p+2+n > len(b) {
				break
			}
			if m == 0xE2 && n >= 16 && bytes.HasPrefix(b[p+4:], []byte("ICC_PROFILE\x00")) {
				parts = append(parts, part{int(b[p+4+12]), b[p+4+14 : p+2+n]})
			}
			p += 2 + n
		}
		if len(parts) == 0 {
			return nil
		}
		var out []byte
		for i := 1; i <= len(parts); i++ {
			found := false
			for _, pt := range parts {
				if pt.seq == i {
					out = append(out, pt.data...)
					found = true
					break
				}
			}
			if !found {
				return nil
			}
		}
		return out
	}
	return nil
}

// isSRGB reports whether the profile is a plain sRGB one, which decoders
// assume anyway, so embedding it would only waste bytes.
func isSRGB(prof []byte) bool {
	return bytes.Contains(prof, []byte("sRGB IEC61966")) || bytes.Contains(prof, []byte("sRGB built-in"))
}

// embedICC inserts prof into an encoded PNG (iCCP after IHDR) or JPEG
// (APP2 chunks after SOI/JFIF). On malformed output it returns out unchanged.
func embedICC(out []byte, f Format, prof []byte) []byte {
	switch f {
	case FormatPNG:
		if len(out) < 33 || !bytes.HasPrefix(out, pngMagic) {
			return out
		}
		var z bytes.Buffer
		zw := zlib.NewWriter(&z)
		zw.Write(prof)
		zw.Close()
		body := append([]byte("icc\x00\x00"), z.Bytes()...)
		chunk := binary.BigEndian.AppendUint32(nil, uint32(len(body)))
		chunk = append(chunk, "iCCP"...)
		chunk = append(chunk, body...)
		chunk = binary.BigEndian.AppendUint32(chunk, crc32.ChecksumIEEE(chunk[4:]))
		return append(append(append([]byte{}, out[:33]...), chunk...), out[33:]...)
	case FormatJPEG:
		if len(out) < 4 || out[0] != 0xFF || out[1] != 0xD8 {
			return out
		}
		at := 2
		if out[2] == 0xFF && out[3] == 0xE0 { // keep JFIF first
			at = 4 + int(binary.BigEndian.Uint16(out[4:]))
		}
		const max = 65519 // payload bytes per APP2 chunk after the 14-byte ICC header
		n := (len(prof) + max - 1) / max
		var segs []byte
		for i := 0; i < n; i++ {
			part := prof[i*max : min((i+1)*max, len(prof))]
			l := 2 + 14 + len(part)
			segs = append(segs, 0xFF, 0xE2, byte(l>>8), byte(l))
			segs = append(segs, "ICC_PROFILE\x00"...)
			segs = append(segs, byte(i+1), byte(n))
			segs = append(segs, part...)
		}
		return append(append(append([]byte{}, out[:at]...), segs...), out[at:]...)
	}
	return out
}
