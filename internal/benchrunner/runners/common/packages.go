// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package common

import "path/filepath"

func DataStreamPath(packageRoot, dataStream string) string {
	return filepath.Join(packageRoot, "data_stream", dataStream)
}
