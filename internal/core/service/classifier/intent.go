package classifier

// Intent is the classified query type (v3.0).
type Intent string

const (
	IntentFact        Intent = "fact"         // 事实型：是什么/多少/何时
	IntentConcept     Intent = "concept"      // 概念型：如何/为什么/原理
	IntentProperNoun  Intent = "proper_noun"  // 专有名词：实体/型号/代码
	IntentHybrid      Intent = "hybrid"       // 多领域/对比/综合
	IntentUnknown     Intent = "unknown"
)

// RoutePlan is the retrieval routing decision from an intent.
type RoutePlan struct {
	Intent           Intent  `json:"intent"`
	Confidence       float64 `json:"confidence"`
	FulltextWeight   float64 `json:"fulltext_weight"`
	VectorWeight     float64 `json:"vector_weight"`
	UseGlobalIndex   bool    `json:"use_global_index"`
	GlobalIndexBoost bool    `json:"global_index_boost"` // 分区检索 + 跨区全局索引补充
	Reason           string  `json:"reason,omitempty"`
}
