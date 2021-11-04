// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package files

import (
	"io/ioutil"

	"github.com/ProtonMail/gopenpgp/v2/crypto"
	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/logger"
)

// VerifySignerConfiguration function verifies if the signer configuration is complete.
func VerifySignerConfiguration() error {
	return nil // TODO
}

// Sign function signs the target file using provided private ket. It creates the {targetFile}.sig file for the given
// {targetFile}.
func Sign(targetFile string) error {
	privateKey := "TODO" // TODO
	passphrase := []byte("TODO")

	data, err := ioutil.ReadFile(targetFile)
	if err != nil {
		return errors.Wrap(err, "can't read the file candidate for signing")
	}

	logger.Debug("Start the signing routine")
	signingKey, err := crypto.NewKeyFromArmored(privateKey)
	if err != nil {
		return errors.Wrap(err, "crypto.NewKeyFromArmored failed")
	}

	unlockedKey, err := signingKey.Unlock(passphrase)
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
	return nil
}
