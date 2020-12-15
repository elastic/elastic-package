// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package elasticsearch

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// NewError returns a new error constructed from the given response body.
// This assumes the body contains a JSON encoded error. If the body cannot
// be parsed then an error is returned that contains the raw body.
func NewError(body []byte) error {
	type ErrorBody struct {
		Error struct {
			Type   string `json:"type"`
			Reason string `json:"reason"`
		} `json:"error"`
	}

	var errBody ErrorBody
	if err := json.NewDecoder(bytes.NewReader(body)).Decode(&errBody); err == nil {
		return fmt.Errorf("elasticsearch error (type=%v): %v", errBody.Error.Type, errBody.Error.Reason)
	}

	// Fall back to including to raw body if it cannot be parsed.
	return fmt.Errorf("elasticsearch error: %v", string(body))
}
