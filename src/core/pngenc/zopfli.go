package pngenc

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"io"

	"github.com/foobaz/go-zopfli/zopfli"
)

// Recompress rewrites the IDAT data of an encoded PNG with Zopfli, typically
// 8–10% smaller than the standard library at best compression, but 10–100×
// slower. It is lossless: the decoded pixels do not change. It returns the
// smaller of the input and the result.
func Recompress(png []byte, iterations int) ([]byte, error) {
	sig := []byte("\x89PNG\r\n\x1a\n")
	if !bytes.HasPrefix(png, sig) {
		return nil, errors.New("pngenc: not a PNG")
	}
	var idat []byte
	type chunk struct {
		typ  string
		data []byte
	}
	var chunks []chunk
	firstIDAT := -1
	for p := len(sig); p+12 <= len(png); {
		n := int(binary.BigEndian.Uint32(png[p:]))
		if p+12+n > len(png) {
			return nil, errors.New("pngenc: truncated chunk")
		}
		typ, data := string(png[p+4:p+8]), png[p+8:p+8+n]
		if typ == "IDAT" {
			if firstIDAT < 0 {
				firstIDAT = len(chunks)
				chunks = append(chunks, chunk{typ: "IDAT"})
			}
			idat = append(idat, data...)
		} else {
			chunks = append(chunks, chunk{typ, data})
		}
		p += 12 + n
	}
	if firstIDAT < 0 {
		return nil, errors.New("pngenc: no IDAT")
	}
	zr, err := zlib.NewReader(bytes.NewReader(idat))
	if err != nil {
		return nil, err
	}
	raw, err := io.ReadAll(zr)
	if err != nil {
		return nil, err
	}
	o := zopfli.DefaultOptions()
	o.NumIterations = iterations
	var z bytes.Buffer
	if err := zopfli.ZlibCompress(&o, raw, &z); err != nil {
		return nil, err
	}
	if z.Len() >= len(idat) {
		return png, nil
	}
	chunks[firstIDAT].data = z.Bytes()
	out := append([]byte(nil), sig...)
	for _, c := range chunks {
		out = binary.BigEndian.AppendUint32(out, uint32(len(c.data)))
		start := len(out)
		out = append(out, c.typ...)
		out = append(out, c.data...)
		out = binary.BigEndian.AppendUint32(out, crc32.ChecksumIEEE(out[start:]))
	}
	return out, nil
}
