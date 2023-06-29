// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package formatter

import (
	"encoding/json"
	"fmt"
)

// JSONFormatter function is responsible for formatting the given JSON input.
// The function is exposed, so it can be used by other internal packages, e.g. to format sample events in docs.
func JSONFormatter(content []byte) ([]byte, bool, error) {
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
