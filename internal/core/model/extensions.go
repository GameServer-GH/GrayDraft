package model

// Extensions is caller-defined metadata attached to a knowledge document.
// Examples: tutorial {"doc_id","section"}; paper {"doi","venue"}.
type Extensions map[string]any
