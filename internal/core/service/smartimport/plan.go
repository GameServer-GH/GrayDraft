package smartimport

// SplitPlan is the 2.0 split decision (macro + micro).
type SplitPlan struct {
	DocAnalysis       DocAnalysis
	Strategy          string // single | multi_segment
	Method            string // rules | llm | rules+llm
	Segments          []PlannedSegment
}

// PlannedSegment is one coordinate-based knowledge unit.
type PlannedSegment struct {
	Content            string
	LineRange          [2]int // 1-based inclusive
	Title              string
	Tags               []string
	ContextEnhancement string
	SearchIntent       string
	SemanticSummary    string
}
