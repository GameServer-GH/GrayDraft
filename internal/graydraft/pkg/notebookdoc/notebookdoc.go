// Package notebookdoc：工作区内单条笔记 HTML/CSS 文档（JSON 文件）。
package notebookdoc

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Document 与前端 NotebookDocument 对齐。
type Document struct {
	HTML string `json:"html"`
	CSS  string `json:"css"`
}

func dir(root string) string {
	return filepath.Join(root, "notebook_documents")
}

func filePath(root, notebookID string) string {
	id := strings.TrimSpace(notebookID)
	return filepath.Join(dir(root), id+".json")
}

// Read 读取文档；不存在时返回 nil, nil。
func Read(root, notebookID string) (*Document, error) {
	p := filePath(root, notebookID)
	b, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var d Document
	if err := json.Unmarshal(b, &d); err != nil {
		return nil, err
	}
	return &d, nil
}

// Write 写入或覆盖文档。
func Write(root, notebookID string, d *Document) error {
	if d == nil {
		return fmt.Errorf("notebookdoc: nil document")
	}
	if err := os.MkdirAll(dir(root), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	return os.WriteFile(filePath(root, notebookID), raw, 0o644)
}

// Delete 删除文档文件（若不存在则忽略）。
func Delete(root, notebookID string) error {
	p := filePath(root, notebookID)
	err := os.Remove(p)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
