// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package system

import (
	"bytes"
	"fmt"
	"strings"
	"sync"

	"github.com/aymerick/raymond"

	"github.com/elastic/elastic-package/internal/certs"
)

type tlsPair struct {
	cert string
	key  string
}

// tlsHelpers returns Handlebars helpers that generate TLS certificates on
// demand. Certificates are cached by domain within a single template
// rendering so that matching tls_cert/tls_key calls return a consistent
// pair.
//
// Template usage (triple-brace to avoid HTML escaping):
//
//	{{{tls_cert "elastic-agent" indent=8}}}
//	{{{tls_key  "elastic-agent" indent=8}}}
//
// The indent hash parameter controls whitespace prepended to continuation
// lines (all lines after the first). Set it to match the column where the
// helper tag appears in the template.
func tlsHelpers() map[string]interface{} {
	var (
		mu    sync.Mutex
		ca    *certs.Issuer
		pairs = make(map[string]*tlsPair)
	)

	getPair := func(domain string) (*tlsPair, error) {
		mu.Lock()
		defer mu.Unlock()

		if p, ok := pairs[domain]; ok {
			return p, nil
		}

		if ca == nil {
			var err error
			ca, err = certs.NewCA()
			if err != nil {
				return nil, fmt.Errorf("generating test CA: %w", err)
			}
		}

		cert, err := ca.Issue(certs.WithName(domain))
		if err != nil {
			return nil, fmt.Errorf("issuing test certificate for %q: %w", domain, err)
		}

		var certBuf, keyBuf bytes.Buffer
		if err := cert.WriteCert(&certBuf); err != nil {
			return nil, fmt.Errorf("encoding test certificate for %q: %w", domain, err)
		}
		if err := cert.WriteKey(&keyBuf); err != nil {
			return nil, fmt.Errorf("encoding test key for %q: %w", domain, err)
		}

		p := &tlsPair{
			cert: strings.TrimRight(certBuf.String(), "\n"),
			key:  strings.TrimRight(keyBuf.String(), "\n"),
		}
		pairs[domain] = p
		return p, nil
	}

	return map[string]interface{}{
		"tls_cert": func(domain string, options *raymond.Options) raymond.SafeString {
			p, err := getPair(domain)
			if err != nil {
				return raymond.SafeString(fmt.Sprintf("ERROR: %s", err))
			}
			return raymond.SafeString(indentAfterFirst(p.cert, hashIndent(options)))
		},
		"tls_key": func(domain string, options *raymond.Options) raymond.SafeString {
			p, err := getPair(domain)
			if err != nil {
				return raymond.SafeString(fmt.Sprintf("ERROR: %s", err))
			}
			return raymond.SafeString(indentAfterFirst(p.key, hashIndent(options)))
		},
	}
}

func hashIndent(options *raymond.Options) int {
	switch n := options.HashProp("indent").(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	default:
		return 0
	}
}

// indentAfterFirst prepends n spaces to every line after the first.
// The first line is left unmodified because its indentation comes from
// the template text preceding the helper tag.
func indentAfterFirst(s string, n int) string {
	if n <= 0 {
		return s
	}
	leftpad := strings.Repeat(" ", n)
	return strings.Join(strings.Split(s, "\n"), "\n"+leftpad)
}
