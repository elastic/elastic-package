// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package promote

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"sort"
	"strings"

	"github.com/go-git/go-billy/v5"
	"github.com/pkg/errors"
)

// hashFiles computes the sha1 hash of a list of files
// First it computes the hash of each of the file's contents then it sorts those
// encoded strings, creates a final string with the sorted file hashes delimited by a newline
// and hashes the final string.
// This effectively produces a hash of a directory
// It is equivalent to: find <path> -type f -exec shasum {} + | awk '{print $1}' | sort | shasum
func hashFiles(filesystem billy.Filesystem, files []string) (string, error) {
	var fileHashes []string
	for _, file := range files {
		if strings.Contains(file, "\n") {
			return "", errors.New("dirhash: filenames with newlines are not supported")
		}

		f, err := filesystem.Open(file)
		if err != nil {
			return "", errors.Wrapf(err, "reading file failed (path: %s)", file)
		}

		c, err := ioutil.ReadAll(f)
		if err != nil {
			return "", errors.Wrapf(err, "reading file content failed (path: %s)", file)
		}

		fileHash := sha1.New()
		fileHash.Write(c)
		fileHashes = append(fileHashes, hex.EncodeToString(fileHash.Sum(nil)))
	}

	sort.Strings(fileHashes)
	var builder strings.Builder
	for _, fileHash := range fileHashes {
		builder.WriteString(fmt.Sprintf("%s\n", fileHash))
	}
	combinedHash := sha1.New()
	combinedHash.Write([]byte(builder.String()))
	return hex.EncodeToString(combinedHash.Sum(nil)), nil
}
