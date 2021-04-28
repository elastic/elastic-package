// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package archetype

import "github.com/elastic/elastic-package/internal/packages"

// DataStreamDescriptor defines configurable properties of the data stream archetype
type DataStreamDescriptor struct {
	Manifest    packages.DataStreamManifest
	PackageRoot string
}
