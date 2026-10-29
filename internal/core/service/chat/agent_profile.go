package chat

import "github.com/graydraft/core/model"

const EduWorkspaceProfile = "edu_workspace_v1"

// ApplyEduProfile constrains a chat request to the education loop.
func ApplyEduProfile(req model.ChatRequest) model.ChatRequest {
	req.AgentProfile = EduWorkspaceProfile
	req.MaxAgentSteps = 8
	req.MemoryMode = "none"
	return req
}
