// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package files

import (
	"io/fs"
	"os"
	"path"
	"strings"
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
	if err != nil {
		return nil, err
	}
	return fileWithPrefix{f, fsys.prefix}, nil
}

func (fsys fsWithPrefix) ReadFile(name string) ([]byte, error) {
	realname, ok := trimPrefix(name, fsys.prefix)
	if !ok {
		return nil, &fs.PathError{Op: "readfile", Path: name, Err: os.ErrNotExist}
	}
	return fs.ReadFile(fsys, realname)
}

func (fsys fsWithPrefix) ReadDir(name string) ([]fs.DirEntry, error) {
	realname, ok := trimPrefix(name, fsys.prefix)
	if !ok {
		return nil, &fs.PathError{Op: "readdir", Path: name, Err: os.ErrNotExist}
	}

	entries, err := fs.ReadDir(fsys.fs, realname)
	if err != nil {
		return nil, err
	}

	// Don't append the prefix in this case because paths should be relative to subdir.
	if realname != "." {
		return entries, nil
	}

	entriesWithPrefix := make([]fs.DirEntry, len(entries))
	for i, entry := range entries {
		entriesWithPrefix[i] = &dirEntryWithPrefix{DirEntry: entry, prefix: fsys.prefix}
	}
	return entriesWithPrefix, nil
}

func (fsys fsWithPrefix) Stat(name string) (fs.FileInfo, error) {
	realname, ok := trimPrefix(name, fsys.prefix)
	if !ok {
		return nil, &fs.PathError{Op: "stat", Path: name, Err: os.ErrNotExist}
	}

	fi, err := fs.Stat(fsys.fs, realname)
	if err != nil {
		return nil, err
	}
	return &fileInfoWithPrefix{FileInfo: fi, prefix: fsys.prefix}, nil
}

type fileWithPrefix struct {
	fs.File
	prefix string
}

func (f fileWithPrefix) Stat() (fs.FileInfo, error) {
	fi, err := f.File.Stat()
	if err != nil {
		return nil, err
	}
	return &fileInfoWithPrefix{FileInfo: fi, prefix: f.prefix}, nil
}

type fileInfoWithPrefix struct {
	fs.FileInfo
	prefix string
}

func (f fileInfoWithPrefix) Name() string {
	return path.Join(f.prefix, f.FileInfo.Name())
}

type dirEntryWithPrefix struct {
	fs.DirEntry
	prefix string
}

func (d dirEntryWithPrefix) Name() string {
	return path.Join(d.prefix, d.DirEntry.Name())
}

func (d dirEntryWithPrefix) Stat() (fs.FileInfo, error) {
	fi, err := d.DirEntry.Info()
	if err != nil {
		return nil, err
	}
	return fileInfoWithPrefix{FileInfo: fi, prefix: d.prefix}, nil
}
