// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package registry

import (
	"bytes"
	"fmt"
	"io"
	"os"

	pgpcrypto "github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/gopenpgp/v2/crypto"

	"github.com/elastic/elastic-package/internal/environment"
)

var (
	disableVerifyPackageSignatureEnv = environment.WithElasticPackagePrefix("DISABLE_VERIFY_PACKAGE_SIGNATURE")
	verifierGPGKeyringEnv            = environment.WithElasticPackagePrefix("VERIFIER_GPG_KEYRING")
)

// VerifyPackageSignatureDisabled reports whether detached PGP verification of
// EPR package zips has been explicitly disabled via the environment.
func VerifyPackageSignatureDisabled() bool {
	return os.Getenv(disableVerifyPackageSignatureEnv) == "true"
}

// LoadVerifierKeyring returns the OpenPGP keyring to use for verifying EPR
// package zip signatures.
//
// Key precedence:
//  1. ELASTIC_PACKAGE_VERIFIER_GPG_KEYRING — if set, the file at that path is
//     read and all armored OpenPGP public keys it contains are loaded into the
//     ring. The file may contain one or more keys concatenated in armored form.
//     When this env var is set the embedded Elastic key is NOT included; users
//     who still need to trust Elastic-signed packages must include the Elastic
//     key in the file alongside their own keys.
//  2. The Elastic public GPG key embedded in the binary (default).
//
// No network access is performed. If the embedded key no longer matches the
// upstream signing key (i.e. Elastic has rotated its key), either upgrade
// elastic-package or set ELASTIC_PACKAGE_VERIFIER_GPG_KEYRING to a file
// containing the new key (and optionally the old one during transition).
func LoadVerifierKeyring() (*crypto.KeyRing, error) {
	override := os.Getenv(verifierGPGKeyringEnv)
	if override != "" {
		data, err := os.ReadFile(override)
		if err != nil {
			return nil, fmt.Errorf("reading GPG keyring %s (set via %s): %w",
				override, verifierGPGKeyringEnv, err)
		}
		ring, err := keyringFromArmoredBytes(data, override)
		if err != nil {
			return nil, fmt.Errorf("%w (set via %s)", err, verifierGPGKeyringEnv)
		}
		return ring, nil
	}
	return keyringFromArmoredBytes(elasticPublicKey, "<embedded>")
}

// keyringFromArmoredBytes parses one or more concatenated armored OpenPGP
// public key blocks from data and returns them as a single *crypto.KeyRing.
// source is used only in error messages.
//
// ReadArmoredKeyRing consumes only the first armor block from a reader because
// armor.Decode wraps it in a bufio.Reader that buffers ahead. Work around this
// by splitting on the armor end-marker before parsing each block independently.
func keyringFromArmoredBytes(data []byte, source string) (*crypto.KeyRing, error) {
	const armorEndMarker = "-----END PGP PUBLIC KEY BLOCK-----"
	blocks := bytes.SplitAfter(data, []byte(armorEndMarker))

	var ring *crypto.KeyRing
	for i, block := range blocks {
		block = bytes.TrimSpace(block)
		if len(block) == 0 {
			continue
		}
		entities, err := pgpcrypto.ReadArmoredKeyRing(bytes.NewReader(block))
		if err != nil {
			return nil, fmt.Errorf("parsing key block %d from %s: %w", i+1, source, err)
		}
		for _, entity := range entities {
			key, err := crypto.NewKeyFromEntity(entity)
			if err != nil {
				return nil, fmt.Errorf("loading key from block %d in %s: %w", i+1, source, err)
			}
			if ring == nil {
				ring, err = crypto.NewKeyRing(key)
			} else {
				err = ring.AddKey(key)
			}
			if err != nil {
				return nil, fmt.Errorf("building keyring from %s: %w", source, err)
			}
		}
	}

	if ring == nil {
		return nil, fmt.Errorf("no OpenPGP keys found in %s", source)
	}
	return ring, nil
}

// VerifyDetachedPGP checks that signatureArmored is a valid detached OpenPGP
// signature over the bytes read from data, verified against keyRing.
func VerifyDetachedPGP(data io.Reader, signatureArmored []byte, keyRing *crypto.KeyRing) error {
	sig, err := crypto.NewPGPSignatureFromArmored(string(signatureArmored))
	if err != nil {
		return fmt.Errorf("reading signature: %w", err)
	}
	if err := keyRing.VerifyDetachedStream(data, sig, crypto.GetUnixTime()); err != nil {
		return fmt.Errorf("signature verification failed: %w", err)
	}
	return nil
}
