// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package elasticsearch_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/elastic-package/internal/elasticsearch"
)

func TestNewError(t *testing.T) {
	const resp = `{
  "error" : {
    "root_cause" : [
      {
        "type" : "parse_exception",
        "reason" : "processor [set] doesn't support one or more provided configuration parameters [fail]",
        "processor_type" : "set"
      }
    ],
    "type" : "parse_exception",
    "reason" : "processor [set] doesn't support one or more provided configuration parameters [fail]",
    "processor_type" : "set"
  },
  "status" : 400
}`

	const expected = `elasticsearch error (type=parse_exception): processor [set] doesn't support one or more provided configuration parameters [fail]
Root cause:
[
  {
    "type": "parse_exception",
    "reason": "processor [set] doesn't support one or more provided configuration parameters [fail]",
    "processor_type": "set",
    "position": {
      "offset": 0,
      "start": 0,
      "end": 0
    }
  }
]`
	err := elasticsearch.NewError([]byte(resp))
	assert.Equal(t, err.Error(), expected)
}
