// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package files

import (
	"io/fs"
	"os"
	"strings"
	"time"
)

// fsWithPrefix is a filesystem whose all the files are prefixed by a given prefix
type fsWithPrefix struct {
	fs     fs.FS
	prefix string
}

// newFSWithPrefix creates a filesystem whose all the files are prefixed by the prefix path
func newFSWithPrefix(fs fs.FS, prefix string) fs.FS {
	return &fsWithPrefix{fs: fs, prefix: prefix}
}

func trimPrefix(name, prefix string) (string, bool) {
	if name == prefix {
		return ".", true
	}
	if name != "." && !strings.HasPrefix(name, prefix+"/") {
		return name, false
	}
	return strings.TrimPrefix(name, prefix+"/"), true
}

func (fsys fsWithPrefix) Open(name string) (fs.File, error) {
	realname, ok := trimPrefix(name, fsys.prefix)
	if !ok {
		return nil, &fs.PathError{Op: "open", Path: name, Err: os.ErrNotExist}
	}
	f, err := fsys.fs.Open(realname)
	return f, setPathErrorPath(name, err)
}

func (fsys fsWithPrefix) ReadFile(name string) ([]byte, error) {
	realname, ok := trimPrefix(name, fsys.prefix)
	if !ok {
		return nil, &fs.PathError{Op: "readfile", Path: name, Err: os.ErrNotExist}
	}
	d, err := fs.ReadFile(fsys.fs, realname)
	return d, setPathErrorPath(name, err)
}

func (fsys fsWithPrefix) ReadDir(name string) ([]fs.DirEntry, error) {
	if name == "." {
		return []fs.DirEntry{prefixDirEntry(fsys.prefix)}, nil
	}

	realname, ok := trimPrefix(name, fsys.prefix)
	if !ok {
		return nil, &fs.PathError{Op: "readdir", Path: name, Err: os.ErrNotExist}
	}

	entries, err := fs.ReadDir(fsys.fs, realname)
	return entries, setPathErrorPath(name, err)
}

func (fsys fsWithPrefix) Stat(name string) (fs.FileInfo, error) {
	realname, ok := trimPrefix(name, fsys.prefix)
	if !ok {
		return nil, &fs.PathError{Op: "stat", Path: name, Err: os.ErrNotExist}
	}

	stat, err := fs.Stat(fsys.fs, realname)
	return stat, setPathErrorPath(name, err)
}

func setPathErrorPath(name string, err error) error {
	if err, ok := err.(*fs.PathError); ok {
		err.Path = name
		return err
	}
	return err
}

type prefixDirEntry string

func (p prefixDirEntry) Name() string               { return string(p) }
func (p prefixDirEntry) Info() (fs.FileInfo, error) { return p, nil }

func (prefixDirEntry) IsDir() bool        { return true }
func (prefixDirEntry) Type() fs.FileMode  { return 0755 }
func (prefixDirEntry) Size() int64        { return 0 }
func (prefixDirEntry) Mode() fs.FileMode  { return 0755 }
func (prefixDirEntry) ModTime() time.Time { return time.Now() }
func (prefixDirEntry) Sys() any           { return nil }
