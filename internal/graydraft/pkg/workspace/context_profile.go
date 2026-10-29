package workspace

import (
	"encoding/json"
	"strings"

	"github.com/graydraft/graydraft/pkg/nexus"
)

// ContextProfileSettings is structured workspace background in settings.json.
type ContextProfileSettings struct {
	Description     string   `json:"description,omitempty"`
	Entities        []string `json:"entities,omitempty"`
	Rules           []string `json:"rules,omitempty"`
	NegativeContext []string `json:"negative_context,omitempty"`
}

// ToNexus converts settings form to Nexus API profile.
func (p ContextProfileSettings) ToNexus() nexus.ContextProfile {
	return nexus.ContextProfile{
		Description:     strings.TrimSpace(p.Description),
		Entities:        cleanStringList(p.Entities),
		Rules:           cleanStringList(p.Rules),
		NegativeContext: cleanStringList(p.NegativeContext),
	}
}

// ContextProfileFromNexus converts Nexus API profile to settings form.
func ContextProfileFromNexus(p nexus.ContextProfile) ContextProfileSettings {
	return contextProfileFromNexus(p)
}

func contextProfileFromNexus(p nexus.ContextProfile) ContextProfileSettings {
	return ContextProfileSettings{
		Description:     p.Description,
		Entities:        append([]string(nil), p.Entities...),
		Rules:           append([]string(nil), p.Rules...),
		NegativeContext: append([]string(nil), p.NegativeContext...),
	}
}

// ReadContextProfile loads structured background from settings.json agentConfig.
func ReadContextProfile(wsRoot string) (ContextProfileSettings, error) {
	raw, err := readSettingsBytes(SettingsPath(wsRoot))
	if err != nil {
		return ContextProfileSettings{}, err
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return ContextProfileSettings{}, err
	}
	acRaw, ok := obj[settingsKeyAgentConfig]
	if !ok {
		return ContextProfileSettings{}, nil
	}
	var ac struct {
		ContextProfile ContextProfileSettings `json:"contextProfile"`
	}
	if err := json.Unmarshal(acRaw, &ac); err != nil {
		return ContextProfileSettings{}, err
	}
	return ac.ContextProfile, nil
}

// MergeContextProfile updates agentConfig.contextProfile in settings blob.
func MergeContextProfile(settings json.RawMessage, profile ContextProfileSettings) json.RawMessage {
	var obj map[string]any
	if len(settings) > 0 {
		_ = json.Unmarshal(settings, &obj)
	}
	if obj == nil {
		obj = map[string]any{}
	}
	ac, _ := obj[settingsKeyAgentConfig].(map[string]any)
	if ac == nil {
		ac = map[string]any{}
	}
	ac["contextProfile"] = profile
	obj[settingsKeyAgentConfig] = ac
	out, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		return settings
	}
	return append(out, '\n')
}

// WorkspaceContextBundle combines structured profile and flattened text for Nexus APIs.
func WorkspaceContextBundle(wsRoot string) (nexus.ContextProfile, string, error) {
	local, err := ReadContextProfile(wsRoot)
	if err != nil {
		return nexus.ContextProfile{}, "", err
	}
	profile := local.ToNexus()
	return profile, profile.WorkspaceContextText(), nil
}

func cleanStringList(in []string) []string {
	out := make([]string, 0, len(in))
	seen := map[string]struct{}{}
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		key := strings.ToLower(s)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, s)
	}
	return out
}
