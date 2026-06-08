// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package servicedeployer

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/elastic/elastic-package/internal/certs"
	"github.com/elastic/elastic-package/internal/logger"
)

// TLSCAPEMProperty is the CustomProperties key for the CA certificate PEM.
const TLSCAPEMProperty = "TLS_CA_PEM"

// generateServiceTLS creates a CA and per-service TLS certificates for
// every compose service that declares a hostname. Cert files are written
// to tlsDir; the CA PEM is stored in svcInfo.CustomProperties so the
// tls_ca Handlebars helper can emit it during config rendering.
func generateServiceTLS(ymlPaths []string, tlsDir string, svcInfo *ServiceInfo) error {
	hostnames, err := extractComposeHostnames(ymlPaths)
	if err != nil {
		return fmt.Errorf("reading compose hostnames: %w", err)
	}
	if len(hostnames) == 0 {
		return nil
	}

	if err := os.MkdirAll(tlsDir, 0755); err != nil {
		return fmt.Errorf("creating TLS directory: %w", err)
	}

	ca, err := certs.NewCA()
	if err != nil {
		return fmt.Errorf("generating test CA: %w", err)
	}
	if err := ca.WriteCertFile(filepath.Join(tlsDir, "ca.crt")); err != nil {
		return fmt.Errorf("writing CA cert: %w", err)
	}

	alias := "svc-" + svcInfo.Name
	for svcName, hostname := range hostnames {
		cert, err := ca.Issue(
			certs.WithName(hostname),
			certs.WithName(alias),
		)
		if err != nil {
			return fmt.Errorf("issuing cert for service %q (hostname %q): %w", svcName, hostname, err)
		}
		if err := cert.WriteCertFile(filepath.Join(tlsDir, svcName+".crt")); err != nil {
			return fmt.Errorf("writing cert for %q: %w", svcName, err)
		}
		if err := cert.WriteKeyFile(filepath.Join(tlsDir, svcName+".key")); err != nil {
			return fmt.Errorf("writing key for %q: %w", svcName, err)
		}
		logger.Debugf("Generated TLS cert for service %q (hostname=%q, alias=%q)", svcName, hostname, alias)
	}

	return storeCAPEM(ca, svcInfo)
}

// loadServiceTLSCA reads an existing ca.crt from tlsDir into
// svcInfo.CustomProperties so the tls_ca helper works in
// --no-provision and --tear-down modes.
func loadServiceTLSCA(tlsDir string, svcInfo *ServiceInfo) error {
	caPath := filepath.Join(tlsDir, "ca.crt")
	data, err := os.ReadFile(caPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("reading CA cert %s: %w", caPath, err)
	}
	if svcInfo.CustomProperties == nil {
		svcInfo.CustomProperties = make(map[string]interface{})
	}
	svcInfo.CustomProperties[TLSCAPEMProperty] = strings.TrimRight(string(data), "\n")
	return nil
}

func storeCAPEM(ca *certs.Issuer, svcInfo *ServiceInfo) error {
	var buf bytes.Buffer
	if err := ca.WriteCert(&buf); err != nil {
		return fmt.Errorf("encoding CA cert: %w", err)
	}
	if svcInfo.CustomProperties == nil {
		svcInfo.CustomProperties = make(map[string]interface{})
	}
	svcInfo.CustomProperties[TLSCAPEMProperty] = strings.TrimRight(buf.String(), "\n")
	return nil
}

// extractComposeHostnames parses compose YAML files and returns a map
// of service name to hostname for every service that declares one.
func extractComposeHostnames(ymlPaths []string) (map[string]string, error) {
	type svc struct {
		Hostname string `yaml:"hostname"`
	}
	type composeFile struct {
		Services map[string]svc `yaml:"services"`
	}

	hostnames := make(map[string]string)
	for _, p := range ymlPaths {
		data, err := os.ReadFile(p)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", p, err)
		}
		var cf composeFile
		if err := yaml.Unmarshal(data, &cf); err != nil {
			return nil, fmt.Errorf("parsing %s: %w", p, err)
		}
		for name, s := range cf.Services {
			if s.Hostname != "" {
				hostnames[name] = s.Hostname
			}
		}
	}
	return hostnames, nil
}
