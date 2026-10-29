package workspace

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/graydraft/graydraft/pkg/nexus"
)

const (
	settingsKeyNexusKbType   = "nexusKbType"
	settingsKeyNexusTenantID = "nexusTenantId"
	settingsKeyNexusKBID     = "nexusKbId"
)

// NexusKbTypeFromRoot reads persisted nexusKbType from settings.json.
func NexusKbTypeFromRoot(wsRoot string) string {
	raw, err := readSettingsBytes(SettingsPath(wsRoot))
	if err != nil {
		return ""
	}
	return NexusKbSettings(json.RawMessage(raw))
}

// NexusKbSettings reads nexusKbType from settings JSON object.
func NexusKbSettings(settings json.RawMessage) string {
	if len(settings) == 0 {
		return ""
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(settings, &m); err != nil {
		return ""
	}
	raw, ok := m[settingsKeyNexusKbType]
	if !ok {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return ""
	}
	return s
}

// MergeNexusKbType returns settings with nexusKbType set (preserves other keys).
func MergeNexusKbType(settings json.RawMessage, kbType string) (json.RawMessage, error) {
	m := map[string]any{}
	if len(settings) > 0 && json.Valid(settings) {
		if err := json.Unmarshal(settings, &m); err != nil {
			return nil, err
		}
	}
	m[settingsKeyNexusKbType] = kbType
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}

// PersistNexusKbType writes nexusKbType into settings.json on disk.
func PersistNexusKbType(root, kbType string) error {
	path := SettingsPath(root)
	raw, err := readSettingsBytes(path)
	if err != nil {
		return err
	}
	merged, err := MergeNexusKbType(json.RawMessage(raw), kbType)
	if err != nil {
		return err
	}
	return WriteSettings(path, merged)
}

// ReadNexusScope loads tenant/kb ids from settings.json.
func ReadNexusScope(root string) (tenantID, kbID string, err error) {
	raw, err := readSettingsBytes(SettingsPath(root))
	if err != nil {
		return "", "", err
	}
	return nexusScopeFromSettings(json.RawMessage(raw))
}

func nexusScopeFromSettings(settings json.RawMessage) (tenantID, kbID string, err error) {
	if len(settings) == 0 {
		return "", "", nil
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(settings, &m); err != nil {
		return "", "", err
	}
	tenantID = jsonString(m[settingsKeyNexusTenantID])
	kbID = jsonString(m[settingsKeyNexusKBID])
	return tenantID, kbID, nil
}

func jsonString(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return ""
	}
	return s
}

// MergeNexusScope returns settings with PaaS scope fields.
func MergeNexusScope(settings json.RawMessage, tenantID, kbID, kbType string) (json.RawMessage, error) {
	m := map[string]any{}
	if len(settings) > 0 && json.Valid(settings) {
		if err := json.Unmarshal(settings, &m); err != nil {
			return nil, err
		}
	}
	m[settingsKeyNexusTenantID] = tenantID
	m[settingsKeyNexusKBID] = kbID
	m[settingsKeyNexusKbType] = kbType
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}

// BindNexusKbType resolves or creates GrayDraft scope for a workspace and persists it.
// ensureScope registers tenant+kb on Core (PaaS API); legacy ensureKB only registers kb_type.
func BindNexusKbType(
	ctx context.Context,
	root string,
	settings json.RawMessage,
	scope nexus.KnowledgeScope,
	ensureScope func(context.Context, nexus.KnowledgeScope) error,
	ensureKB func(context.Context, string, string) error,
) (kbType string, settingsOut json.RawMessage, err error) {
	kbType = NexusKbSettings(settings)
	if kbType == "" {
		kbType = scope.StoragePartition
	}
	if kbType == "" {
		return "", settings, fmt.Errorf("empty nexus kb_type")
	}
	scope.StoragePartition = kbType
	if scope.TenantID == "" || scope.KBID == "" {
		return "", settings, fmt.Errorf("empty nexus tenant/kb scope")
	}
	if ensureScope != nil {
		if err := ensureScope(ctx, scope); err != nil {
			return "", settings, err
		}
	} else if ensureKB != nil {
		desc := fmt.Sprintf("GrayDraft workspace partition %s", kbType)
		if err := ensureKB(ctx, kbType, desc); err != nil {
			return "", settings, err
		}
	}
	settingsOut, err = MergeNexusScope(settings, scope.TenantID, scope.KBID, kbType)
	if err != nil {
		return "", settings, err
	}
	onDiskKb := NexusKbSettings(settings)
	onDiskTenant, onDiskKbID, _ := nexusScopeFromSettings(settings)
	if onDiskKb != kbType || onDiskTenant != scope.TenantID || onDiskKbID != scope.KBID {
		if err := WriteSettings(SettingsPath(root), settingsOut); err != nil {
			return "", settings, err
		}
	} else if _, err := os.Stat(SettingsPath(root)); err != nil {
		if err := WriteSettings(SettingsPath(root), settingsOut); err != nil {
			return "", settings, err
		}
	}
	return kbType, settingsOut, nil
}
