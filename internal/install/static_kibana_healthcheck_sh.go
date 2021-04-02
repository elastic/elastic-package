// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package install

const kibanaHealthcheckSh = `#!/bin/bash

curl -s -f http://127.0.0.1:5601/login | grep kbn-injected-metadata 2>&1 >/dev/null
curl -s -f -u elastic:changeme http://elasticsearch:9200/_cat/shards | grep .security | grep STARTED
`
