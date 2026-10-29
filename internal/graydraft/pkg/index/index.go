// Package index：notebooks 索引（可扩展为扫描 .graydraft / notebooks-index）。
package index

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// NotebookMeta 与前端 src/types/notebook.d.ts 对齐。
type NotebookMeta struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	UpdatedAt    string `json:"updatedAt"`
	ThumbnailURL string `json:"thumbnailUrl,omitempty"`
}

// Service 基于 dataDir 下的 notebooks.json 持久化。
type Service struct {
	dataDir string
}

func NewService(dataDir string) *Service {
	return &Service{dataDir: dataDir}
}

func (s *Service) path() string {
	return filepath.Join(s.dataDir, "notebooks.json")
}

// ListNotebooks 返回索引；文件不存在时返回空列表（由调用方负责初始化目录与文件，如 workspace.Open）。
func (s *Service) ListNotebooks(ctx context.Context) ([]NotebookMeta, error) {
	_ = ctx
	if err := os.MkdirAll(s.dataDir, 0o755); err != nil {
		return nil, err
	}
	b, err := os.ReadFile(s.path())
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
		return []NotebookMeta{}, nil
	}
	var out []NotebookMeta
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Service) writeNotebooks(list []NotebookMeta) error {
	raw, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path(), raw, 0o644)
}

// CreateNotebook 在索引头部插入一条并写回 notebooks.json。
func (s *Service) CreateNotebook(ctx context.Context, title string) (NotebookMeta, error) {
	_ = ctx
	list, err := s.ListNotebooks(ctx)
	if err != nil {
		return NotebookMeta{}, err
	}
	t := strings.TrimSpace(title)
	if t == "" {
		t = "未命名笔记"
	}
	meta := NotebookMeta{
		ID:        fmt.Sprintf("nb-%d", time.Now().UnixNano()),
		Title:     t,
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	out := append([]NotebookMeta{meta}, list...)
	if err := s.writeNotebooks(out); err != nil {
		return NotebookMeta{}, err
	}
	return meta, nil
}

// DeleteNotebook 从索引中移除指定 id；不存在时返回错误。
func (s *Service) DeleteNotebook(ctx context.Context, id string) error {
	_ = ctx
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("missing id")
	}
	list, err := s.ListNotebooks(ctx)
	if err != nil {
		return err
	}
	out := list[:0]
	for _, n := range list {
		if n.ID != id {
			out = append(out, n)
		}
	}
	if len(out) == len(list) {
		return fmt.Errorf("notebook not found")
	}
	return s.writeNotebooks(out)
}

// RenameNotebook 更新指定 id 的标题；不存在时返回错误。
func (s *Service) RenameNotebook(ctx context.Context, id string, title string) (NotebookMeta, error) {
	_ = ctx
	id = strings.TrimSpace(id)
	if id == "" {
		return NotebookMeta{}, fmt.Errorf("missing id")
	}
	t := strings.TrimSpace(title)
	if t == "" {
		t = "未命名笔记"
	}
	list, err := s.ListNotebooks(ctx)
	if err != nil {
		return NotebookMeta{}, err
	}
	for i := range list {
		if list[i].ID != id {
			continue
		}
		list[i].Title = t
		list[i].UpdatedAt = time.Now().UTC().Format(time.RFC3339)
		updated := list[i]
		if err := s.writeNotebooks(list); err != nil {
			return NotebookMeta{}, err
		}
		return updated, nil
	}
	return NotebookMeta{}, fmt.Errorf("notebook not found")
}
