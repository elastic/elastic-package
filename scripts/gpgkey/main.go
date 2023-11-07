// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package main

import (
	"log"
	"os"
	"strings"

	"github.com/ProtonMail/gopenpgp/v2/crypto"
)

const (
	signerPassphraseEnv = "ELASTIC_PACKAGE_SIGNER_PASSPHRASE"
	privateKeyPathEnv   = "ELASTIC_PACKAGE_SIGNER_PRIVATE_KEYFILE"
)

func main() {
	passphrase := readPassphrase()
	privateKeyPath, publicKeyPath := getKeyPaths()
	rsaKeyArmor, rsaPublicKeyArmor := genKey(passphrase)

	err := os.WriteFile(privateKeyPath, rsaKeyArmor, 0644)
	if err != nil {
		log.Fatal(err)
	}
	err = os.WriteFile(publicKeyPath, rsaPublicKeyArmor, 0644)
	if err != nil {
		log.Fatal(err)
	}
}

func genKey(passphrase []byte) ([]byte, []byte) {
	const (
		name    = "Elastic Package Test"
		rsaBits = 2048
	)

	rsaKey, err := crypto.GenerateKey(name, "", "rsa", rsaBits)
	if err != nil {
		log.Fatal(err)
	}
	rsaKey, err = rsaKey.Lock(passphrase)
	if err != nil {
		log.Fatal(err)
	}

	rsaKeyArmor, err := rsaKey.Armor()
	if err != nil {
		log.Fatal(err)
	}
	rsaPublicKeyArmor, err := rsaKey.GetArmoredPublicKey()
	if err != nil {
		log.Fatal(err)
	}

	return []byte(rsaKeyArmor), []byte(rsaPublicKeyArmor)
}

func readPassphrase() []byte {
	passphrase := os.Getenv(signerPassphraseEnv)
	if passphrase == "" {
		log.Fatalf("Environment variable %s empty or not set", signerPassphraseEnv)
	}
	return []byte(passphrase)
}

func getKeyPaths() (string, string) {
	privateKeyPath := os.Getenv(privateKeyPathEnv)
	if privateKeyPath == "" {
		log.Fatalf("Environment variable %s empty or not set", privateKeyPathEnv)
	}

	publicKeyPath := strings.ReplaceAll(privateKeyPath, "private", "public")
	if privateKeyPath == publicKeyPath {
		log.Fatalf("The path indicated in %s is expected to contain \"private\", found: %s", privateKeyPathEnv, privateKeyPath)
	}

	return privateKeyPath, publicKeyPath
}
