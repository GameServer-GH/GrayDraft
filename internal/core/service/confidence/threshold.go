package confidence

import (
	"strings"

	"github.com/graydraft/core/model"
)

const (
	gateThresholdDefault = 0.20
	gateThresholdFact    = 0.25 // 事实类：宁缺毋滥
	gateThresholdConcept = 0.15 // 开放类：允许更多上下文
	gateFloor            = 0.12
	gateCeiling          = 0.32
)

// RetrievalGateThreshold returns the base minimum composite confidence for kb_only / hybrid RAG gate.
func RetrievalGateThreshold(queryIntent string) float64 {
	return retrievalGateBase(queryIntent)
}

func retrievalGateBase(queryIntent string) float64 {
	switch strings.TrimSpace(strings.ToLower(queryIntent)) {
	case "fact":
		return gateThresholdFact
	case "concept":
		return gateThresholdConcept
	default:
		return gateThresholdDefault
	}
}

// RetrievalGateThresholdDynamic adjusts base threshold using top-chunk quality and title signals.
// 动态门限 ≈ 意图基础分 − 质量/标题加成（工作区文档质量高则略放宽）。
func RetrievalGateThresholdDynamic(queryIntent string, chunks []model.Chunk) float64 {
	base := retrievalGateBase(queryIntent)
	if len(chunks) == 0 {
		return base
	}
	top := chunks[0]
	if top.QualityScore >= 0.80 {
		base -= 0.05
	} else if top.QualityScore >= 0.65 {
		base -= 0.03
	}
	if top.TitleMatched {
		base -= 0.02
	}
	if top.EntityMatched {
		base -= 0.02
	}
	if base < gateFloor {
		return gateFloor
	}
	if base > gateCeiling {
		return gateCeiling
	}
	return base
}
