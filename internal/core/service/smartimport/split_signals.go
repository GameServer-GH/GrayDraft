package smartimport

// contentLooksMixed reports multi-topic structure from skeleton scan (no keyword scoring).
func contentLooksMixed(sk DocumentSkeleton) bool {
	switch sk.Analysis.DocType {
	case "综合合集", "分段文档", "问答集":
		return true
	default:
		return false
	}
}
