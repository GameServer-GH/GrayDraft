package agent

import (
	"crypto/sha256"
	"encoding/hex"
	"math/big"
	"regexp"
	"strconv"
	"strings"
)

// Only explicit same-unit equivalences are checked. Different measurements,
// uncertainty intervals, and implicit relationships must not be compared.
var explicitScaleEquality = regexp.MustCompile(`(?:约为|大约为|近似为|约|大约|近似|≈|~)?\s*[±+-]?\s*([0-9]+(?:\.[0-9]+)?)\s*[×x*]\s*10\s*\^?\s*([+-]?[0-9]+)\s*([^\n（()，,；;。]{1,24})[（(，,]?\s*(即|等于|相当于)\s*(?:约为|大约为|近似为|约|大约|近似|≈|~)?\s*[±+-]?\s*([0-9]+(?:\.[0-9]+)?)\s*(千万|百万|十亿|亿|万|千)\s*([^\n）)，,；;。]{1,24})`)

type NumericOperationReview struct {
	CanonicalUnit      string `json:"canonical_unit,omitempty"`
	Rule               string `json:"rule"`
	Status             string `json:"status"`
	Action             string `json:"action"`
	LeftCoefficient    string `json:"left_coefficient"`
	LeftExponent       string `json:"left_power10"`
	LeftUnit           string `json:"left_unit"`
	RightCoefficient   string `json:"right_coefficient"`
	RightScale         string `json:"right_scale"`
	RightUnit          string `json:"right_unit"`
	Relation           string `json:"relation"`
	Approximate        bool   `json:"approximate"`
	LeftMagnitude      string `json:"left_magnitude_rational,omitempty"`
	RightMagnitude     string `json:"right_magnitude_rational,omitempty"`
	RelativeDifference string `json:"relative_difference_rational,omitempty"`
	Tolerance          string `json:"relative_tolerance_rational"`
	Scope              string `json:"scope"`
}

func normalizeReviewText(text string) string {
	return strings.NewReplacer("⁰", "0", "¹", "1", "²", "2", "³", "3", "⁴", "4", "⁵", "5", "⁶", "6", "⁷", "7", "⁸", "8", "⁹", "9", "⁻", "-", "⁺", "+", "$", "", "\\,", " ", "**", "").Replace(text)
}
func canonicalReviewUnit(s string) string {
	s = strings.TrimSpace(s)
	for _, p := range [][2]string{{`M_\odot`, "太阳质量"}, {"M⊙", "太阳质量"}, {"M☉", "太阳质量"}, {"kg", "千克"}, {"m", "米"}, {"s", "秒"}} {
		if s == p[0] {
			return p[1]
		}
	}
	return s
}

func reviewNumericOperations(answer string) ([]NumericOperationReview, bool) {
	matches := explicitScaleEquality.FindAllStringSubmatch(normalizeReviewText(answer), 17)
	truncated := len(matches) > 16
	if truncated {
		matches = matches[:16]
	}
	var records []NumericOperationReview
	scales := map[string]int64{"千": 1000, "万": 10000, "百万": 1000000, "千万": 10000000, "亿": 100000000, "十亿": 1000000000}
	for _, m := range matches {
		approximate := strings.Contains(m[0], "约") || strings.Contains(m[0], "近似") || strings.Contains(m[0], "≈") || strings.Contains(m[0], "~") || m[4] == "相当于"
		r := NumericOperationReview{Rule: "same_unit_scale_v2", Status: "not_covered", Action: "review", LeftCoefficient: m[1], LeftExponent: m[2], LeftUnit: strings.TrimSpace(m[3]), RightCoefficient: m[5], RightScale: m[6], RightUnit: strings.TrimSpace(m[7]), Relation: m[4], Approximate: approximate, Tolerance: "1/1000000", Scope: "candidate_internal_magnitude_only"}
		// Keep audit data bounded without guessing the value of oversized operands.
		if len(r.LeftCoefficient) > 64 || len(r.RightCoefficient) > 64 || len(r.LeftExponent) > 8 {
			r.LeftCoefficient = "oversized"
			r.RightCoefficient = "oversized"
			r.LeftExponent = "oversized"
			records = append(records, r)
			continue
		}
		if canonicalReviewUnit(r.LeftUnit) != canonicalReviewUnit(r.RightUnit) {
			r.Status = "unit_binding_unverified"
			records = append(records, r)
			continue
		}
		r.CanonicalUnit = canonicalReviewUnit(r.LeftUnit)
		a, okA := new(big.Rat).SetString(m[1])
		b, okB := new(big.Rat).SetString(m[5])
		exponent, err := strconv.Atoi(m[2])
		if !okA || !okB || err != nil || exponent < -300 || exponent > 300 {
			records = append(records, r)
			continue
		}
		absExponent := exponent
		if absExponent < 0 {
			absExponent = -absExponent
		}
		power := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(absExponent)), nil)
		factor := new(big.Rat).SetInt(power)
		if exponent < 0 {
			factor.Inv(factor)
		}
		left := new(big.Rat).Mul(a, factor)
		right := new(big.Rat).Mul(b, new(big.Rat).SetInt64(scales[m[6]]))
		r.LeftMagnitude = left.RatString()
		r.RightMagnitude = right.RatString()
		difference := new(big.Rat).Abs(new(big.Rat).Sub(left, right))
		maximum := new(big.Rat).Abs(left)
		absRight := new(big.Rat).Abs(right)
		if absRight.Cmp(maximum) > 0 {
			maximum = absRight
		}
		relative := new(big.Rat)
		if maximum.Sign() != 0 {
			relative.Quo(difference, maximum)
		}
		r.RelativeDifference = relative.RatString()
		r.Status = "consistent_magnitude"
		r.Action = "none"
		if approximate {
			r.Status = "approximation_requires_review"
			r.Action = "review"
		} else if relative.Cmp(new(big.Rat).SetFrac64(1, 1000000)) > 0 {
			r.Status = "contradicted_magnitude"
			r.Action = "reject"
		}
		records = append(records, r)
	}
	return records, truncated
}

func numericConsistencyIssues(answer string) []string {
	records, _ := reviewNumericOperations(answer)
	var issues []string
	for _, r := range records {
		if r.Action == "reject" {
			issues = append(issues, "explicit_same_unit_scale_mismatch")
		}
	}
	return issues
}

func candidateReviewHash(answer string) string {
	sum := sha256.Sum256([]byte(answer))
	return hex.EncodeToString(sum[:])
}
