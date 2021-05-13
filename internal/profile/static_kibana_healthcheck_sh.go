// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package profile

import "path/filepath"

const kibanaHealthcheckSh = `#!/bin/bash

set -e

curl -s -f http://127.0.0.1:5601/login | grep kbn-injected-metadata 2>&1 >/dev/null
curl -s -f -u elastic:changeme "http://elasticsearch:9200/_cat/indices/.security-*?h=health" | grep -v red
`

// KibanaHealthCheckFile is the config file for the Elastic Package registry
const KibanaHealthCheckFile configFile = "healthcheck.sh"

// newKibanaHealthCheck returns a Managed Config
func newKibanaHealthCheck(_ string, profilePath string) (*simpleFile, error) {
	return &simpleFile{
		name: string(KibanaHealthCheckFile),
		path: filepath.Join(profilePath, string(KibanaHealthCheckFile)),
		body: kibanaHealthcheckSh,
	}, nil
}
