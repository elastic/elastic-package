// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package registry

import (
	"fmt"
	"io"
	"os"

	"github.com/ProtonMail/gopenpgp/v2/crypto"

	"github.com/elastic/elastic-package/internal/environment"
)

var (
	disableVerifyPackageSignatureEnv = environment.WithElasticPackagePrefix("DISABLE_VERIFY_PACKAGE_SIGNATURE")
	verifierPublicKeyfileEnv         = environment.WithElasticPackagePrefix("VERIFIER_PUBLIC_KEYFILE")
)

// VerifyPackageSignatureDisabled reports whether detached PGP verification of
// EPR package zips has been explicitly disabled via the environment.
func VerifyPackageSignatureDisabled() bool {
	return os.Getenv(disableVerifyPackageSignatureEnv) == "true"
}

// LoadVerifierPublicKey returns the armored OpenPGP public key to use for
// verifying EPR package zip signatures.
//
// Key precedence:
//  1. ELASTIC_PACKAGE_VERIFIER_PUBLIC_KEYFILE — if set, the file at that path
//     is read and returned. An error is returned if the path is unset-but-set
//     to empty, or the file cannot be read.
//  2. The Elastic public GPG key embedded in the binary (default).
//
// No network access is performed. If the embedded key no longer matches the
// upstream signing key (i.e. Elastic has rotated its key), the caller should
// either upgrade elastic-package or set ELASTIC_PACKAGE_VERIFIER_PUBLIC_KEYFILE
// to a manually-vetted copy of the new key.
func LoadVerifierPublicKey() ([]byte, error) {
	override := os.Getenv(verifierPublicKeyfileEnv)
	if override != "" {
		data, err := os.ReadFile(override)
		if err != nil {
			return nil, fmt.Errorf("reading verifier public key from %s (set via %s): %w",
				override, verifierPublicKeyfileEnv, err)
		}
		return data, nil
	}
	return ElasticPublicKey(), nil
}

// VerifyDetachedPGP checks that signatureArmored is a valid detached OpenPGP
// signature over the bytes read from data, using publicKeyArmored.
func VerifyDetachedPGP(data io.Reader, signatureArmored []byte, publicKeyArmored []byte) error {
	pubKey, err := crypto.NewKeyFromArmored(string(publicKeyArmored))
	if err != nil {
		return fmt.Errorf("reading public key: %w", err)
	}
	keyRing, err := crypto.NewKeyRing(pubKey)
	if err != nil {
		return fmt.Errorf("building key ring: %w", err)
	}
	sig, err := crypto.NewPGPSignatureFromArmored(string(signatureArmored))
	if err != nil {
		return fmt.Errorf("reading signature: %w", err)
	}
	if err := keyRing.VerifyDetachedStream(data, sig, crypto.GetUnixTime()); err != nil {
		return fmt.Errorf("signature verification failed: %w", err)
	}
	return nil
}
