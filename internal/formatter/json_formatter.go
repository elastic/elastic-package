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

type JSONFormatter interface {
	Format([]byte) ([]byte, bool, error)
	Encode(doc any) ([]byte, error)
}

func JSONFormatterBuilder(specVersion semver.Version) JSONFormatter {
	if specVersion.LessThan(semver.MustParse("2.12.0")) {
		return &jsonFormatterWithHTMLEncoding{}
	}

	return &jsonFormatter{}
}

// jsonFormatterWithHTMLEncoding function is responsible for formatting the given JSON input.
// It encodes special HTML characters.
type jsonFormatterWithHTMLEncoding struct{}

func (jsonFormatterWithHTMLEncoding) Format(content []byte) ([]byte, bool, error) {
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

func (jsonFormatterWithHTMLEncoding) Encode(doc any) ([]byte, error) {
	return json.MarshalIndent(doc, "", "    ")
}

// jsonFormatter function is responsible for formatting the given JSON input.
type jsonFormatter struct{}

func (jsonFormatter) Format(content []byte) ([]byte, bool, error) {
	var formatted bytes.Buffer
	err := json.Indent(&formatted, content, "", "    ")
	if err != nil {
		return nil, false, fmt.Errorf("formatting JSON document failed: %w", err)
	}

	return formatted.Bytes(), bytes.Equal(content, formatted.Bytes()), nil
}

func (jsonFormatter) Encode(doc any) ([]byte, error) {
	var formatted bytes.Buffer
	enc := json.NewEncoder(&formatted)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "    ")

	err := enc.Encode(doc)
	if err != nil {
		return nil, err
	}

	// Trimming to be consistent with MarshalIndent, that seems to trim the result.
	return bytes.TrimSpace(formatted.Bytes()), nil
}
