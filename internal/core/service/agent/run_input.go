package agent

import (
	"encoding/json"
	"strconv"
	"strings"
)

// Persist the structured input in the existing prompt column and queue payload.
// Decoding happens before memory construction; options never become question text.
const runInputPrefix = "\x1egraydraft-input-v1\n"

type InputConfig struct {
	TopK             int    `json:"top_k,omitempty"`
	ChatMode         string `json:"chat_mode,omitempty"`
	WorkspaceContext string `json:"workspace_context,omitempty"`
}
type runInput struct {
	Question string      `json:"question"`
	Config   InputConfig `json:"config"`
}

func EncodeRunInput(question string, config InputConfig) []byte {
	b, _ := json.Marshal(runInput{Question: question, Config: config})
	return append([]byte(runInputPrefix), b...)
}
func DecodeRunInput(raw []byte) (string, InputConfig) {
	if strings.HasPrefix(string(raw), runInputPrefix) {
		var in runInput
		if json.Unmarshal(raw[len(runInputPrefix):], &in) == nil {
			return in.Question, in.Config
		}
	}
	text := string(raw)
	if strings.HasPrefix(text, "top_k=") || strings.HasPrefix(text, "chat_mode=") || strings.HasPrefix(text, "workspace_context:") {
		config := InputConfig{}
		lines := strings.Split(text, "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "top_k=") {
				config.TopK, _ = strconv.Atoi(strings.TrimPrefix(line, "top_k="))
			}
			if strings.HasPrefix(line, "chat_mode=") {
				config.ChatMode = strings.TrimPrefix(line, "chat_mode=")
			}
		}
		return CurrentUserQuery(text), config
	}
	return text, InputConfig{}
}
