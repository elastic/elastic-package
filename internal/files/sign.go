// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package files

import (
	"fmt"
	"os"

	"github.com/ProtonMail/gopenpgp/v2/armor"
	"github.com/ProtonMail/gopenpgp/v2/constants"
	"github.com/ProtonMail/gopenpgp/v2/crypto"

	"github.com/elastic/elastic-package/internal/environment"
	"github.com/elastic/elastic-package/internal/logger"
)

const signatureComment = "Signed with elastic-package (using GopenPGP: https://gopenpgp.org)"

var (
	signerPrivateKeyfileEnv = environment.WithElasticPackagePrefix("SIGNER_PRIVATE_KEYFILE")
	signerPassphraseEnv     = environment.WithElasticPackagePrefix("SIGNER_PASSPHRASE")
)

type SignOptions struct {
	PackageName    string
	PackageVersion string
}

// VerifySignerConfiguration function verifies if the signer configuration is complete.
func VerifySignerConfiguration() error {
	signerPrivateKeyfile := os.Getenv(signerPrivateKeyfileEnv)
	if signerPrivateKeyfile == "" {
		return fmt.Errorf("signer private keyfile is required. Please define it with environment variable %s", signerPrivateKeyfileEnv)
	}

	_, err := os.Stat(signerPrivateKeyfile)
	if err != nil {
		return fmt.Errorf("can't access the signer private keyfile: %w", err)
	}

	signerPassphrase := os.Getenv(signerPassphraseEnv)
	if signerPassphrase == "" {
		return fmt.Errorf("signer passphrase is required. Please define it with environment variable %s", signerPassphraseEnv)
	}
	return nil
}

// Sign function signs the target file using provided private ket. It creates the {targetFile}.sig file for the given
// {targetFile}.
func Sign(targetFile string, options SignOptions) error {
	signerPrivateKeyfile := os.Getenv(signerPrivateKeyfileEnv)
	logger.Debugf("Read signer private keyfile: %s", signerPrivateKeyfile)
	signerPrivateKey, err := os.ReadFile(signerPrivateKeyfile)
	if err != nil {
		return fmt.Errorf("can't read the signer private keyfile (path: %s): %w", signerPrivateKeyfile, err)
	}

	signerPassphrase := []byte(os.Getenv(signerPassphraseEnv))

	logger.Debug("Start the signing routine")
	signingKey, err := crypto.NewKeyFromArmored(string(signerPrivateKey))
	if err != nil {
		return fmt.Errorf("crypto.NewKeyFromArmored failed: %w", err)
	}

	unlockedKey, err := signingKey.Unlock(signerPassphrase)
	if err != nil {
		return fmt.Errorf("signingKey.Unlock failed: %w", err)
	}
	defer unlockedKey.ClearPrivateParams()

	keyRing, err := crypto.NewKeyRing(unlockedKey)
	if err != nil {
		return fmt.Errorf("crypto.NewKeyRing failed: %w", err)
	}

	messageReader, err := os.Open(targetFile)
	if err != nil {
		return fmt.Errorf("os.Open failed (targetFile: %s): %w", targetFile, err)
	}
	defer messageReader.Close()

	signature, err := keyRing.SignDetachedStream(messageReader)
	if err != nil {
		return fmt.Errorf("keyRing.SignDetached failed: %w", err)
	}

	armoredSignature, err := armor.ArmorWithTypeAndCustomHeaders(signature.Data, constants.PGPSignatureHeader,
		fmt.Sprintf("%s-%s", options.PackageName, options.PackageVersion), signatureComment)
	if err != nil {
		return fmt.Errorf("signature.GetArmored failed: %w", err)
	}

	logger.Debug("Signature generated for the target file, writing the .sig file")
	targetSigFile := targetFile + ".sig"
	err = os.WriteFile(targetSigFile, []byte(armoredSignature), 0644)
	if err != nil {
		return fmt.Errorf("can't write the signature file: %w", err)
	}

	logger.Infof("Signature file written: %s", targetSigFile)
	return nil
}
