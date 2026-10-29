package embedder

import (
	"crypto/sha256"
	"encoding/binary"
	"math"
	"math/rand"
)

// PseudoEmbed generates a deterministic normalized vector (dev / fallback).
// Matches Worker Python logic: SHA256 seed → Gaussian → L2 normalize.
func PseudoEmbed(text string, dim int) []float32 {
	sum := sha256.Sum256([]byte(text))
	seed := int64(binary.BigEndian.Uint64(sum[:8]))
	rng := rand.New(rand.NewSource(seed))

	vec := make([]float32, dim)
	var norm float64
	for i := 0; i < dim; i++ {
		vec[i] = float32(rng.NormFloat64())
		norm += float64(vec[i]) * float64(vec[i])
	}
	if norm > 0 {
		scale := float32(1.0 / math.Sqrt(norm))
		for i := range vec {
			vec[i] *= scale
		}
	}
	return vec
}
