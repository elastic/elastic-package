// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package files

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/ProtonMail/gopenpgp/v2/armor"
	"github.com/ProtonMail/gopenpgp/v2/constants"
	"github.com/ProtonMail/gopenpgp/v2/crypto"
	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/logger"
)

const (
	signerPrivateKeyfileEnv = "ELASTIC_PACKAGE_SIGNER_PRIVATE_KEYFILE"
	signerPassphraseEnv     = "ELASTIC_PACKAGE_SIGNER_PASSPHRASE"

	signatureComment = "Signed with elastic-package (using GopenPGP: https://gopenpgp.org)"
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
		return errors.Wrap(err, "can't access the signer private keyfile")
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
	signerPrivateKey, err := ioutil.ReadFile(signerPrivateKeyfile)
	if err != nil {
		return errors.Wrapf(err, "can't read the signer private keyfile (path: %s)", signerPrivateKeyfile)
	}

	signerPassphrase := []byte(os.Getenv(signerPassphraseEnv))

	logger.Debug("Start the signing routine")
	signingKey, err := crypto.NewKeyFromArmored(string(signerPrivateKey))
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

	messageReader, err := os.Open(targetFile)
	if err != nil {
		return errors.Wrapf(err, "os.Open failed (targetFile: %s)", targetFile)
	}
	defer messageReader.Close()

	signature, err := keyRing.SignDetachedStream(messageReader)
	if err != nil {
		return errors.Wrap(err, "keyRing.SignDetached failed")
	}

	armoredSignature, err := armor.ArmorWithTypeAndCustomHeaders(signature.Data, constants.PGPSignatureHeader,
		fmt.Sprintf("%s-%s", options.PackageName, options.PackageVersion), signatureComment)
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
