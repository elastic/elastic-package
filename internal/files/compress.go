// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package files

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// Zip function creates the .zip archive from the source path (built package content).
func Zip(sourcePath, destinationFile string) error {
	out, err := os.Create(destinationFile)
	if err != nil {
		return err
	}
	defer out.Close()

	folderName := folderNameFromFileName(destinationFile)
	z := zip.NewWriter(out)
	err = addFSWithPrefix(z, os.DirFS(sourcePath), folderName)
	if err != nil {
		return fmt.Errorf("failed to add files to package zip: %w", err)
	}
	// No need to z.Flush() because z.Close() already does it.
	err = z.Close()
	if err != nil {
		return fmt.Errorf("failed to write data to zip file: %w", err)
	}
	return nil
}

// folderNameFromFileName returns the folder name from the destination file,
// Based on mholt/archiver: https://github.com/mholt/archiver/blob/d35d4ce7c5b2411973fb7bd96ca1741eb011011b/archiver.go#L397
func folderNameFromFileName(filename string) string {
	base := filepath.Base(filename)
	firstDot := strings.LastIndex(base, ".")
	if firstDot > -1 {
		return base[:firstDot]
	}
	return base
}

// addFSWithPrefix adds the files from fs.FS to the archive adding as a first folder of the zip the given package (e.g. nginx-1.0.0)
// Implementation based on AddFS method from archive/zip package in Go 1.20+
// https://cs.opensource.google/go/go/+/refs/tags/go1.25.4:src/archive/zip/writer.go;l=503
func addFSWithPrefix(zw *zip.Writer, fsys fs.FS, prefix string) error {
	return fs.WalkDir(fsys, ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if name == "." {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if !d.IsDir() && !info.Mode().IsRegular() {
			return errors.New("zip: cannot add non-regular file")
		}
		h, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}

		// Set the path containing the base folder within the zip file
		h.Name = path.Join(prefix, name)
		if d.IsDir() {
			h.Name += "/"
		}
		h.Method = zip.Deflate
		fw, err := zw.CreateHeader(h)
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		f, err := fsys.Open(name)
		if err != nil {
			return fmt.Errorf("failed to open file: %w", err)
		}
		defer f.Close()
		_, err = io.Copy(fw, f)
		return err
	})
}
