// Package export：静态站导出任务（占位：登记 job，后续 worker pool）。
package export

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Service 管理导出任务元数据目录。
type Service struct {
	jobsDir string
}

func NewService(dataDir string) *Service {
	return &Service{jobsDir: filepath.Join(dataDir, "export_jobs")}
}

type Job struct {
	ID        string `json:"id"`
	ProjectID string `json:"projectId"`
	Status    string `json:"status"`
}

func newID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

// Start 创建导出任务并落盘，返回 jobId。
func (s *Service) Start(projectID string) (string, error) {
	if projectID == "" {
		return "", fmt.Errorf("export: empty projectId")
	}
	if err := os.MkdirAll(s.jobsDir, 0o755); err != nil {
		return "", err
	}
	id, err := newID()
	if err != nil {
		return "", err
	}
	job := Job{ID: id, ProjectID: projectID, Status: "queued"}
	raw, err := json.MarshalIndent(job, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(s.jobsDir, id+".json"), raw, 0o644); err != nil {
		return "", err
	}
	return id, nil
}
