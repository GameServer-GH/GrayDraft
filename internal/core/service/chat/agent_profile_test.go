package chat

import (
	"testing"

	"github.com/graydraft/core/model"
)

func TestApplyEduProfile(t *testing.T) {
	r := ApplyEduProfile(model.ChatRequest{Query: "q"})
	if r.AgentProfile != EduWorkspaceProfile || r.MaxAgentSteps != 8 || r.MemoryMode != "none" {
		t.Fatalf("%+v", r)
	}
}
