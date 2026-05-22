// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package registry

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"

	pgpopenpgp "github.com/ProtonMail/go-crypto/openpgp"
	pgparmor "github.com/ProtonMail/go-crypto/openpgp/armor"
	"github.com/ProtonMail/gopenpgp/v2/crypto"

	"github.com/elastic/elastic-package/internal/environment"
)

var (
	disableVerifyPackageSignatureEnv = environment.WithElasticPackagePrefix("VERIFIER_DISABLE")
	verifierGPGKeyringEnv            = environment.WithElasticPackagePrefix("VERIFIER_GPG_KEYRING")
)

// verifyPackageSignatureDisabled reports whether detached PGP verification of
// EPR package zips has been explicitly disabled via the environment.
func verifyPackageSignatureDisabled() bool {
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
func loadVerifierKeyring() (*crypto.KeyRing, error) {
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
// It loops calling armor.Decode on a shared bufio.Reader — the same pattern
// used by encoding/pem with pem.Decode — so each armor block is parsed
// independently without any string splitting.
func keyringFromArmoredBytes(data []byte, source string) (*crypto.KeyRing, error) {
	r := bufio.NewReader(bytes.NewReader(data))

	var ring *crypto.KeyRing
	for i := 1; ; i++ {
		block, err := pgparmor.Decode(r)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("parsing key block %d from %s: %w", i, source, err)
		}

		entities, err := pgpopenpgp.ReadKeyRing(block.Body)
		if err != nil {
			return nil, fmt.Errorf("reading key block %d from %s: %w", i, source, err)
		}
		for _, entity := range entities {
			key, err := crypto.NewKeyFromEntity(entity)
			if err != nil {
				return nil, fmt.Errorf("loading key from block %d in %s: %w", i, source, err)
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

// verifyDetachedPGP checks that signatureArmored is a valid detached OpenPGP
// signature over the bytes read from data, verified against keyRing.
func verifyDetachedPGP(data io.Reader, signatureArmored []byte, keyRing *crypto.KeyRing) error {
	sig, err := crypto.NewPGPSignatureFromArmored(string(signatureArmored))
	if err != nil {
		return fmt.Errorf("reading signature: %w", err)
	}
	if err := keyRing.VerifyDetachedStream(data, sig, crypto.GetUnixTime()); err != nil {
		return fmt.Errorf("signature verification failed: %w", err)
	}
	return nil
}
