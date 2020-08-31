// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package formatter

import (
	"encoding/json"

	"github.com/pkg/errors"
)

func jsonFormatter(content []byte) ([]byte, bool, error) {
	var rawMessage json.RawMessage
	err := json.Unmarshal(content, &rawMessage)
	if err != nil {
		return nil, false, errors.Wrap(err, "unmarshalling JSON file failed")
	}

	formatted, err := json.MarshalIndent(&rawMessage, "", "    ")
	if err != nil {
		return nil, false, errors.Wrap(err, "marshalling JSON raw message failed")
	}
	return formatted, string(content) == string(formatted), nil
}
