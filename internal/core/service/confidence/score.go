package confidence

import "github.com/graydraft/core/model"

// FromImportAnalysis maps analyzer confidence to import eligibility.
func FromImportAnalysis(conf float64, minRequired float64) (importAllowed bool, effective float64) {
	effective = clamp(conf)
	return effective >= minRequired, effective
}

// FromRetrieval estimates answer reliability from retrieved chunks.
func FromRetrieval(chunks []model.Chunk, total int) float64 {
	if len(chunks) == 0 || total == 0 {
		return 0
	}
	var sum float64
	var max float64
	for _, c := range chunks {
		s := c.Score
		if s < 0 {
			s = 0
		}
		if s > 1 {
			s = 1
		}
		sum += s
		if s > max {
			max = s
		}
	}
	avg := sum / float64(len(chunks))
	// 有结果但分数偏低时限制上限
	c := 0.55*max + 0.45*avg
	if len(chunks) < 2 && total < 3 {
		c *= 0.85
	}
	return clamp(c)
}

func clamp(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
