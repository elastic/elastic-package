// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package formatter

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/Masterminds/semver/v3"
)

func JSONFormatterBuilder(specVersion semver.Version) func([]byte) ([]byte, bool, error) {
	if specVersion.LessThan(semver.MustParse("2.12.0")) {
		return jsonFormatterWithHTMLEncoding
	}

	return jsonFormatter
}

// jsonFormatterWithHTMLEncoding function is responsible for formatting the given JSON input.
// It encodes special HTML characters.
func jsonFormatterWithHTMLEncoding(content []byte) ([]byte, bool, error) {
	var rawMessage json.RawMessage
	err := json.Unmarshal(content, &rawMessage)
	if err != nil {
		return nil, false, fmt.Errorf("unmarshalling JSON file failed: %w", err)
	}

	formatted, err := json.MarshalIndent(&rawMessage, "", "    ")
	if err != nil {
		return nil, false, fmt.Errorf("marshalling JSON raw message failed: %w", err)
	}
	return formatted, string(content) == string(formatted), nil
}

// jsonFormatter function is responsible for formatting the given JSON input.
func jsonFormatter(content []byte) ([]byte, bool, error) {
	var formatted bytes.Buffer
	err := json.Indent(&formatted, content, "", "    ")
	if err != nil {
		return nil, false, fmt.Errorf("formatting JSON document failed: %w", err)
	}

	return formatted.Bytes(), bytes.Equal(content, formatted.Bytes()), nil
}
