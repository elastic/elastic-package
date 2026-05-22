// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

// fetch downloads the Elastic public GPG key and writes it to
// internal/files/elastic-gpg-key.asc. It is intended to be called via
// go generate ./internal/files/... when the upstream key rotates.
//
// Usage: go run ./internal/files/gpgkey/fetch
package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/ProtonMail/gopenpgp/v2/crypto"
)

const (
	upstreamKeyURL = "https://packages.elasticsearch.org/GPG-KEY-elasticsearch"
	keyFileName    = "elastic-gpg-key.asc"
)

func main() {
	// Locate the target file relative to this source file so it works
	// regardless of where go generate is invoked from.
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		log.Fatal("could not determine source file path")
	}
	// thisFile is .../internal/registry/gpgkey/fetch/main.go
	// target is  .../internal/registry/elastic-gpg-key.asc
	targetPath := filepath.Join(filepath.Dir(thisFile), "..", "..", keyFileName)

	oldFingerprint := readCurrentFingerprint(targetPath)

	log.Printf("Fetching key from %s ...", upstreamKeyURL)
	keyBytes := fetchKey(upstreamKeyURL)

	newFingerprint := mustFingerprint(keyBytes)

	if err := os.WriteFile(targetPath, keyBytes, 0o644); err != nil {
		log.Fatalf("writing %s: %v", targetPath, err)
	}

	fmt.Printf("old_fingerprint=%s\n", oldFingerprint)
	fmt.Printf("new_fingerprint=%s\n", newFingerprint)

	if oldFingerprint == newFingerprint {
		log.Print("Key unchanged.")
	} else {
		log.Printf("Key updated: %s -> %s", oldFingerprint, newFingerprint)
		log.Printf("Review the new key carefully before committing %s.", keyFileName)
	}
}

// fetchKey downloads and returns armored GPG key bytes from url. It validates
// that the bytes parse as an armored OpenPGP public key before returning.
func fetchKey(url string) []byte {
	resp, err := http.Get(url) //nolint:noctx
	if err != nil {
		log.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		log.Fatalf("GET %s: unexpected status %d", url, resp.StatusCode)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("reading response body: %v", err)
	}
	// Validate before writing.
	mustFingerprint(data)
	return data
}

// mustFingerprint parses keyBytes as an armored OpenPGP public key and returns
// the primary key fingerprint. Calls log.Fatal on any parse error.
func mustFingerprint(keyBytes []byte) string {
	key, err := crypto.NewKeyFromArmored(string(keyBytes))
	if err != nil {
		log.Fatalf("parsing GPG key: %v", err)
	}
	return strings.ToUpper(key.GetFingerprint())
}

// readCurrentFingerprint returns the fingerprint of the key currently stored
// at path, or "<none>" if the file doesn't exist or cannot be parsed.
func readCurrentFingerprint(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return "<none>"
	}
	key, err := crypto.NewKeyFromArmored(string(data))
	if err != nil {
		return "<unparseable>"
	}
	return strings.ToUpper(key.GetFingerprint())
}
