package workspace

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/graydraft/graydraft/pkg/index"
)

const settingsFile = "settings.json"

// SettingsPath 为工作区内 settings.json 路径。
func SettingsPath(root string) string {
	return filepath.Join(root, settingsFile)
}

// Open 确保工作区目录存在；若新建则写入空 notebooks.json、空 settings.json 与文档子目录。
// 返回 (root, created, err)。created 为 true 表示本次调用新建了工作区目录。
func Open(dataDir, rawKey string) (root string, created bool, err error) {
	key, err := NormalizeKey(rawKey)
	if err != nil {
		return "", false, err
	}
	root = Root(dataDir, key)
	_, statErr := os.Stat(root)
	if os.IsNotExist(statErr) {
		if err = os.MkdirAll(filepath.Join(root, "notebook_documents"), 0o755); err != nil {
			return "", false, err
		}
		if err = os.WriteFile(filepath.Join(root, "notebooks.json"), []byte("[]\n"), 0o644); err != nil {
			return "", false, err
		}
		emptySettings, _ := json.MarshalIndent(map[string]any{}, "", "  ")
		if err = os.WriteFile(SettingsPath(root), append(emptySettings, '\n'), 0o644); err != nil {
			return "", false, err
		}
		return root, true, nil
	}
	if statErr != nil {
		return "", false, statErr
	}
	return root, false, nil
}

// Snapshot 汇总打开工作区后前端一次恢复所需的数据。
func Snapshot(ctx context.Context, root string) (notebooks []index.NotebookMeta, settings json.RawMessage, err error) {
	idx := index.NewService(root)
	list, err := idx.ListNotebooks(ctx)
	if err != nil {
		return nil, nil, err
	}
	raw, err := readSettingsBytes(SettingsPath(root))
	if err != nil {
		return nil, nil, err
	}
	if len(raw) == 0 || string(raw) == "null" {
		raw = []byte("{}")
	}
	if !json.Valid(raw) {
		raw = []byte("{}")
	}
	return list, json.RawMessage(raw), nil
}

func readSettingsBytes(path string) ([]byte, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []byte("{}"), nil
		}
		return nil, err
	}
	return b, nil
}
