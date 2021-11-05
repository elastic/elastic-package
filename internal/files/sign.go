// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package files

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/ProtonMail/gopenpgp/v2/crypto"
	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/logger"
)

const (
	signerPrivateKeyEnv = "ELASTIC_PACKAGE_SIGNER_PRIVATE_KEY"
	signerPassphraseEnv = "ELASTIC_PACKAGE_SIGNER_PASSPHRASE"
)

// VerifySignerConfiguration function verifies if the signer configuration is complete.
func VerifySignerConfiguration() error {
	signerPrivateKey := os.Getenv(signerPrivateKeyEnv)
	if signerPrivateKey == "" {
		return fmt.Errorf("signer private key is required. Please define it with environment variable %s", signerPrivateKeyEnv)
	}

	signerPassphrase := os.Getenv(signerPassphraseEnv)
	if signerPassphrase == "" {
		return fmt.Errorf("signer passphrase is required. Please define it with environment variable %s", signerPassphraseEnv)
	}
	return nil
}

// Sign function signs the target file using provided private ket. It creates the {targetFile}.sig file for the given
// {targetFile}.
func Sign(targetFile string) error {
	signerPrivateKey := os.Getenv(signerPrivateKeyEnv)
	signerPassphrase := []byte(os.Getenv(signerPassphraseEnv))

	data, err := ioutil.ReadFile(targetFile)
	if err != nil {
		return errors.Wrap(err, "can't read the file candidate for signing")
	}

	logger.Debug("Start the signing routine")
	signingKey, err := crypto.NewKeyFromArmored(signerPrivateKey)
	if err != nil {
		return errors.Wrap(err, "crypto.NewKeyFromArmored failed")
	}

	unlockedKey, err := signingKey.Unlock(signerPassphrase)
	if err != nil {
		return errors.Wrap(err, "signingKey.Unlock failed")
	}
	defer unlockedKey.ClearPrivateParams()

	keyRing, err := crypto.NewKeyRing(unlockedKey)
	if err != nil {
		return errors.Wrap(err, "crypto.NewKeyRing failed")
	}

	message := crypto.NewPlainMessage(data)
	if err != nil {
		return errors.Wrap(err, "crypto.NewPlainMessageFromString failed")
	}

	signature, err := keyRing.SignDetached(message)
	if err != nil {
		return errors.Wrap(err, "keyRing.SignDetached failed")
	}

	armoredSignature, err := signature.GetArmored()
	if err != nil {
		return errors.Wrap(err, "signature.GetArmored failed")
	}

	logger.Debug("Signature generated for the target file, writing the .sig file")
	targetSigFile := targetFile + ".sig"
	err = ioutil.WriteFile(targetSigFile, []byte(armoredSignature), 0644)
	if err != nil {
		return errors.Wrap(err, "can't write the signature file")
	}

	logger.Infof("Signature file written: %s", targetSigFile)
	return nil
}
