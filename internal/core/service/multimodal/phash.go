package multimodal

import (
	"bytes"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"math/bits"
	"strings"
)

// computePHash returns a 16-hex perceptual hash for images; empty for non-images.
func computePHash(raw []byte, mime string) string {
	if !strings.HasPrefix(mime, "image/") {
		return ""
	}
	img, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return ""
	}
	bounds := img.Bounds()
	const size = 32
	pixels := make([]float64, size*size)
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			sx := bounds.Min.X + (x * bounds.Dx() / size)
			sy := bounds.Min.Y + (y * bounds.Dy() / size)
			r, g, b, _ := img.At(sx, sy).RGBA()
			pixels[y*size+x] = 0.299*float64(r>>8) + 0.587*float64(g>>8) + 0.114*float64(b>>8)
		}
	}
	const block = 4
	var blocks [8][8]float64
	for by := 0; by < 8; by++ {
		for bx := 0; bx < 8; bx++ {
			var sum float64
			for y := 0; y < block; y++ {
				for x := 0; x < block; x++ {
					sum += pixels[(by*block+y)*size+(bx*block+x)]
				}
			}
			blocks[by][bx] = sum / float64(block*block)
		}
	}
	mean := 0.0
	for by := 0; by < 8; by++ {
		for bx := 0; bx < 8; bx++ {
			mean += blocks[by][bx]
		}
	}
	mean /= 64

	var hash uint64
	for i := 0; i < 64; i++ {
		by := i / 8
		bx := i % 8
		if blocks[by][bx] >= mean {
			hash |= 1 << uint(i)
		}
	}
	return formatUint64Hex(hash)
}

func formatUint64Hex(v uint64) string {
	const hexdigits = "0123456789abcdef"
	out := make([]byte, 16)
	for i := 15; i >= 0; i-- {
		out[i] = hexdigits[v&0xf]
		v >>= 4
	}
	return string(out)
}

func hammingDistance(a, b string) int {
	if len(a) != 16 || len(b) != 16 {
		return 64
	}
	var av, bv uint64
	for i := 0; i < 16; i++ {
		av = (av << 4) | hexVal(a[i])
		bv = (bv << 4) | hexVal(b[i])
	}
	return int(bits.OnesCount64(av ^ bv))
}

func hexVal(c byte) uint64 {
	switch {
	case c >= '0' && c <= '9':
		return uint64(c - '0')
	case c >= 'a' && c <= 'f':
		return uint64(c - 'a' + 10)
	case c >= 'A' && c <= 'F':
		return uint64(c - 'A' + 10)
	default:
		return 0
	}
}
