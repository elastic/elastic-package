// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package system

// Document corresponds to the logs or metrics event stored in the data stream.
type Document struct {
	Error *struct {
		Message string
	}
}
