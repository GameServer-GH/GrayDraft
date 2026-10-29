// Package bundle：.graydraft 包读写（占位，后续实现 zip/fs 布局）。
package bundle

import "fmt"

// ErrNotImplemented 表示尚未实现的包操作。
var ErrNotImplemented = fmt.Errorf("bundle: not implemented")

// Project 占位类型。
type Project struct {
	Path string
}

// Open 打开本地 .graydraft 路径（后续实现）。
func Open(path string) (*Project, error) {
	if path == "" {
		return nil, fmt.Errorf("bundle: empty path")
	}
	return &Project{Path: path}, nil
}
