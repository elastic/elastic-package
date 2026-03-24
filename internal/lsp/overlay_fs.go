// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package lsp

import (
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

// overlayFS serves package files from disk while letting open editor buffers
// override specific files with unsaved content.
type overlayFS struct {
	base      fs.FS
	overrides map[string]string
}

func newOverlayFS(packageRoot string, overrides map[string]string) fs.FS {
	return &overlayFS{
		base:      os.DirFS(packageRoot),
		overrides: overrides,
	}
}

func (o *overlayFS) Open(name string) (fs.File, error) {
	normalized, err := normalizeFSPath(name)
	if err != nil {
		return nil, err
	}

	if text, ok := o.overrides[normalized]; ok {
		return &virtualFile{
			Reader: strings.NewReader(text),
			info: virtualFileInfo{
				name: path.Base(normalized),
				size: int64(len(text)),
			},
		}, nil
	}

	return o.base.Open(normalized)
}

type virtualFile struct {
	*strings.Reader
	info virtualFileInfo
}

func (f *virtualFile) Stat() (fs.FileInfo, error) {
	return f.info, nil
}

func (f *virtualFile) Close() error {
	return nil
}

type virtualFileInfo struct {
	name string
	size int64
}

func (i virtualFileInfo) Name() string {
	return i.name
}

func (i virtualFileInfo) Size() int64 {
	return i.size
}

func (i virtualFileInfo) Mode() fs.FileMode {
	return 0o444
}

func (i virtualFileInfo) ModTime() time.Time {
	return time.Time{}
}

func (i virtualFileInfo) IsDir() bool {
	return false
}

func (i virtualFileInfo) Sys() any {
	return nil
}

func relativeFSPath(packageRoot, filePath string) (string, bool) {
	if packageRoot == "" {
		return "", false
	}

	relPath, err := filepath.Rel(packageRoot, filePath)
	if err != nil {
		return "", false
	}
	relPath = filepath.ToSlash(relPath)
	if relPath == "." || relPath == "" || relPath == ".." || strings.HasPrefix(relPath, "../") {
		return "", false
	}
	return relPath, true
}

func normalizeFSPath(name string) (string, error) {
	switch name {
	case "", ".":
		return ".", nil
	}

	normalized := path.Clean(strings.TrimPrefix(name, "./"))
	if !fs.ValidPath(normalized) {
		return "", &fs.PathError{Op: "open", Path: name, Err: fs.ErrInvalid}
	}
	return normalized, nil
}
