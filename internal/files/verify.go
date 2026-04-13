// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package files

import (
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/ProtonMail/gopenpgp/v2/crypto"

	"github.com/elastic/elastic-package/internal/environment"
)

var (
	verifyPackageSignatureEnv = environment.WithElasticPackagePrefix("VERIFY_PACKAGE_SIGNATURE")
	verifierPublicKeyfileEnv  = environment.WithElasticPackagePrefix("VERIFIER_PUBLIC_KEYFILE")
)

// PackageSignatureVerificationFromEnv reports whether detached PGP verification should run
// for registry package downloads. When verify is true, publicKeyPath is the path to an
// armored public key and has been checked for existence. A non-nil err means the environment
// is inconsistent (e.g. verify enabled but no key path or inaccessible file).
func PackageSignatureVerificationFromEnv() (verify bool, publicKeyPath string, err error) {
	raw := os.Getenv(verifyPackageSignatureEnv)
	if raw == "" {
		return false, "", nil
	}
	verify, err = strconv.ParseBool(raw)
	if err != nil {
		return false, "", fmt.Errorf("parse %s=%q: %w", verifyPackageSignatureEnv, raw, err)
	}
	if !verify {
		return false, "", nil
	}
	publicKeyPath = os.Getenv(verifierPublicKeyfileEnv)
	if publicKeyPath == "" {
		return true, "", fmt.Errorf("%s is true but %s is not set", verifyPackageSignatureEnv, verifierPublicKeyfileEnv)
	}
	if _, err := os.Stat(publicKeyPath); err != nil {
		return true, "", fmt.Errorf("can't access verifier public keyfile (path: %s): %w", publicKeyPath, err)
	}
	return true, publicKeyPath, nil
}

// VerifyDetachedPGP checks that signatureArmored is a valid detached OpenPGP signature over
// the bytes read from data, using the armored publicKeyArmored.
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
