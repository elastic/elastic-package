// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

// fetch downloads the Elastic public GPG key and writes it to
// internal/registry/elastic-gpg-key.asc. It is intended to be called via
// go generate ./internal/registry/... when the upstream key rotates.
//
// Usage: go run ./internal/registry/gpgkey/fetch
package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/ProtonMail/gopenpgp/v2/crypto"
)

const (
	upstreamKeyURL = "https://packages.elasticsearch.org/GPG-KEY-elasticsearch"
	// go generate runs with the working directory set to the package directory
	// (internal/registry/), so keyFileName is relative to that.
	keyFileName = "elastic-gpg-key.asc"
)

func main() {
	oldFingerprint := readCurrentFingerprint(keyFileName)

	log.Printf("Fetching key from %s ...", upstreamKeyURL)
	keyBytes, newFingerprint := fetchKey(upstreamKeyURL)

	if err := os.WriteFile(keyFileName, keyBytes, 0o644); err != nil {
		log.Fatalf("writing %s: %v", keyFileName, err)
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

// fetchKey downloads the armored GPG key from url, validates it parses as an
// OpenPGP public key, and returns the raw bytes and its fingerprint.
func fetchKey(url string) ([]byte, string) {
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
	return data, mustFingerprint(data)
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
