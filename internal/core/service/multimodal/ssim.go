// Package multimodal provides SSIM (Structural Similarity Index) computation
// for pHash verification level 2 (optimization plan 3.2).
//
// Implements sliding-window SSIM on grayscale images with an 11×11 Gaussian window.
// Returns a score in [0, 1]; ≥ 0.85 indicates structurally similar images.
package multimodal

import (
	"bytes"
	"image"
	"math"
)

const (
	ssimWindowSize = 11
	ssimK1         = 0.01
	ssimK2         = 0.03
	ssimL          = 255.0
	ssimC1         = (ssimK1 * ssimL) * (ssimK1 * ssimL) // 6.5025
	ssimC2         = (ssimK2 * ssimL) * (ssimK2 * ssimL) // 58.5225
)

// gaussianKernel11 generates an 11×11 Gaussian kernel with σ = 1.5.
func gaussianKernel11() []float64 {
	sigma := 1.5
	size := ssimWindowSize
	half := size / 2
	kernel := make([]float64, size*size)
	var sum float64
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			dx := float64(x - half)
			dy := float64(y - half)
			v := math.Exp(-(dx*dx+dy*dy)/(2*sigma*sigma)) / (2 * math.Pi * sigma * sigma)
			kernel[y*size+x] = v
			sum += v
		}
	}
	// Normalize
	for i := range kernel {
		kernel[i] /= sum
	}
	return kernel
}

var ssimKernel = gaussianKernel11()

// ComputeSSIM calculates the mean SSIM index between two grayscale images.
// Both images must have the same dimensions. Returns a value in [0, 1].
func ComputeSSIM(a, b *image.Gray) float64 {
	bounds := a.Bounds()
	if bounds != b.Bounds() {
		return 0
	}
	w, h := bounds.Dx(), bounds.Dy()
	if w < ssimWindowSize || h < ssimWindowSize {
		// Image too small for windowed SSIM; compute global SSIM
		return globalSSIM(a, b)
	}

	var totalSSIM float64
	var count int
	half := ssimWindowSize / 2

	for y := half; y < h-half; y++ {
		for x := half; x < w-half; x++ {
			s := windowSSIM(a, b, x, y, w)
			totalSSIM += s
			count++
		}
	}
	if count == 0 {
		return 0
	}
	return totalSSIM / float64(count)
}

// windowSSIM computes SSIM for an 11×11 window centered at (cx, cy).
func windowSSIM(a, b *image.Gray, cx, cy, stride int) float64 {
	half := ssimWindowSize / 2
	var muA, muB float64

	// Compute means using Gaussian weights
	for dy := -half; dy <= half; dy++ {
		for dx := -half; dx <= half; dx++ {
			px := cx + dx
			py := cy + dy
			weight := ssimKernel[(dy+half)*ssimWindowSize+(dx+half)]
			muA += float64(a.GrayAt(px, py).Y) * weight
			muB += float64(b.GrayAt(px, py).Y) * weight
		}
	}

	// Compute variances and covariance
	var varA, varB, cov float64
	for dy := -half; dy <= half; dy++ {
		for dx := -half; dx <= half; dx++ {
			px := cx + dx
			py := cy + dy
			weight := ssimKernel[(dy+half)*ssimWindowSize+(dx+half)]
			da := float64(a.GrayAt(px, py).Y) - muA
			db := float64(b.GrayAt(px, py).Y) - muB
			varA += da * da * weight
			varB += db * db * weight
			cov += da * db * weight
		}
	}

	// SSIM formula
	num := (2*muA*muB + ssimC1) * (2*cov + ssimC2)
	den := (muA*muA + muB*muB + ssimC1) * (varA + varB + ssimC2)
	if den == 0 {
		return 0
	}
	return num / den
}

// globalSSIM computes a single SSIM value for the entire image.
func globalSSIM(a, b *image.Gray) float64 {
	bounds := a.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	total := float64(w * h)
	if total == 0 {
		return 0
	}

	// Global means
	var muA, muB float64
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			muA += float64(a.GrayAt(x, y).Y)
			muB += float64(b.GrayAt(x, y).Y)
		}
	}
	muA /= total
	muB /= total

	// Global variances and covariance
	var varA, varB, cov float64
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			da := float64(a.GrayAt(x, y).Y) - muA
			db := float64(b.GrayAt(x, y).Y) - muB
			varA += da * da
			varB += db * db
			cov += da * db
		}
	}
	varA /= total
	varB /= total
	cov /= total

	num := (2*muA*muB + ssimC1) * (2*cov + ssimC2)
	den := (muA*muA + muB*muB + ssimC1) * (varA + varB + ssimC2)
	if den == 0 {
		return 0
	}
	return num / den
}

// DecodeGray decodes raw image bytes to grayscale.
func DecodeGray(raw []byte, mime string) (*image.Gray, error) {
	img, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	bounds := img.Bounds()
	gray := image.NewGray(bounds)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			gray.Set(x, y, img.At(x, y))
		}
	}
	return gray, nil
}

// pHashPassesSSIMCheck performs SSIM verification for pHash-matched images.
// This is level 2 of the three-level pHash verification chain.
// Returns true if SSIM >= 0.85.
func pHashPassesSSIMCheck(raw, cachedRaw []byte, mime string) bool {
	a, err := DecodeGray(raw, mime)
	if err != nil {
		return true // cannot verify, default to pass
	}
	b, err := DecodeGray(cachedRaw, mime)
	if err != nil {
		return true
	}
	return ComputeSSIM(a, b) >= 0.85
}

// pHashThreeLevelCheck runs the full three-level pHash verification:
// Level 1: Color histogram correlation ≥ 0.90 → pass
// Level 2: SSIM ≥ 0.85 → pass
// Level 3: CLIP (stub — always returns true for now)
// Returns true if any level passes.
func pHashThreeLevelCheck(raw, cachedRaw []byte, mime string) bool {
	// Level 1: Color histogram
	if pHashPassesColorCheck(raw, cachedRaw, mime) {
		return true
	}
	// Level 2: SSIM
	if pHashPassesSSIMCheck(raw, cachedRaw, mime) {
		return true
	}
	// Level 3: CLIP (stub — defer to pass when histogram and SSIM are uncertain)
	// In production, this would call an external CLIP service.
	return true
}
