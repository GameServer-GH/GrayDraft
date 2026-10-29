// Package model defines IngestStage pipeline phases and state machine rules (V1-07, V1-08).
package model

import "fmt"

// IngestStage enumerates the pipeline phases.
type IngestStage string

const (
	StageAccepted       IngestStage = "accepted"
	StageParsed         IngestStage = "parsed"
	StageMediaExtracted IngestStage = "media_extracted"
	StageChunked        IngestStage = "chunked"
	StageEmbedded       IngestStage = "embedded"
	StageIndexed        IngestStage = "indexed"
	StageQA             IngestStage = "qa"
)

// StageOrder defines the linear pipeline progression.
var StageOrder = []IngestStage{
	StageAccepted,
	StageParsed,
	StageMediaExtracted,
	StageChunked,
	StageEmbedded,
	StageIndexed,
	StageQA,
}

// NextStage returns the following stage in the pipeline, or empty string if terminal.
func (s IngestStage) NextStage() IngestStage {
	for i, st := range StageOrder {
		if st == s && i+1 < len(StageOrder) {
			return StageOrder[i+1]
		}
	}
	return ""
}

// SkipStage returns the stage after skipping one intermediate stage (e.g., skip media_extracted).
func (s IngestStage) SkipStage() IngestStage {
	for i, st := range StageOrder {
		if st == s && i+2 < len(StageOrder) {
			return StageOrder[i+2]
		}
	}
	return ""
}

// allowed transitions for the stage state machine.
// A stage may transition to itself (retry), to the next expected stage, or to a skip stage.
var stageTransitions = map[IngestStage]map[IngestStage]bool{
	StageAccepted:       {StageAccepted: true, StageParsed: true},
	StageParsed:         {StageParsed: true, StageMediaExtracted: true, StageChunked: true},
	StageMediaExtracted: {StageMediaExtracted: true, StageChunked: true},
	StageChunked:        {StageChunked: true, StageEmbedded: true},
	StageEmbedded:       {StageEmbedded: true, StageIndexed: true},
	StageIndexed:        {StageIndexed: true, StageQA: true},
	StageQA:             {StageQA: true},
}

// CanStageTransition reports whether moving from → to is permitted.
func CanStageTransition(from, to IngestStage) bool {
	if from == "" {
		return to == StageAccepted
	}
	m, ok := stageTransitions[from]
	if !ok {
		return false
	}
	return m[to]
}

// StageSubjectName returns the default JetStream subject for a stage.
func StageSubjectName(stage IngestStage) string {
	return fmt.Sprintf("graydraft.ingest.%s", stage)
}

// AllStageSubjects returns all default stage subjects.
func AllStageSubjects() []string {
	subjects := make([]string, len(StageOrder))
	for i, s := range StageOrder {
		subjects[i] = StageSubjectName(s)
	}
	return subjects
}
