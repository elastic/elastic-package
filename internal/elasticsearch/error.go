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
type ErrorBody struct {
	Error struct {
		RootCause []struct {
			Type          string   `json:"type"`
			Reason        string   `json:"reason"`
			ProcessorType string   `json:"processor_type,omitempty"`
			ScriptStack   []string `json:"script_stack,omitempty"`
			Script        string   `json:"script,omitempty"`
			Lang          string   `json:"lang,omitempty"`
			Position      struct {
				Offset int `json:"offset"`
				Start  int `json:"start"`
				End    int `json:"end"`
			} `json:"position,omitempty"`
			Suppressed []struct {
				Type          string `json:"type"`
				Reason        string `json:"reason"`
				ProcessorType string `json:"processor_type"`
			} `json:"suppressed,omitempty"`
		} `json:"root_cause,omitempty"`
		Type          string   `json:"type"`
		Reason        string   `json:"reason"`
		ProcessorType string   `json:"processor_type,omitempty"`
		ScriptStack   []string `json:"script_stack,omitempty"`
		Script        string   `json:"script,omitempty"`
		Lang          string   `json:"lang,omitempty"`
		Position      struct {
			Offset int `json:"offset"`
			Start  int `json:"start"`
			End    int `json:"end"`
		} `json:"position,omitempty"`
		CausedBy struct {
			Type     string `json:"type"`
			Reason   string `json:"reason"`
			CausedBy struct {
				Type   string      `json:"type"`
				Reason interface{} `json:"reason"`
			} `json:"caused_by,omitempty"`
		} `json:"caused_by,omitempty"`
		Suppressed []struct {
			Type          string `json:"type"`
			Reason        string `json:"reason"`
			ProcessorType string `json:"processor_type"`
		} `json:"suppressed,omitempty"`
	} `json:"error"`
	Status int `json:"status"`
}

func NewError(body []byte) error {
	var errBody ErrorBody
	if err := json.NewDecoder(bytes.NewReader(body)).Decode(&errBody); err == nil {
		if len(errBody.Error.RootCause) > 0 {
			rootCause, _ := json.MarshalIndent(errBody.Error.RootCause, "", "  ")
			return fmt.Errorf("elasticsearch error (type=%v): %v\nRoot cause:\n%v", errBody.Error.Type,
				errBody.Error.Reason, string(rootCause))
		}
		return fmt.Errorf("elasticsearch error (type=%v): %v", errBody.Error.Type, errBody.Error.Reason)
	}
	// Fall back to including to raw body if it cannot be parsed.
	return fmt.Errorf("elasticsearch error: %v", string(body))
}
