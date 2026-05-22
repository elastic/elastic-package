// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

//go:generate go run ./gpgkey/fetch

package registry

import _ "embed"

//go:embed elastic-gpg-key.asc
var elasticPublicKey []byte
