// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package tools

import _ "embed"

// The embedded example_readme is an example of a high-quality integration readme, following the static template archetype,
// which will help the LLM follow an example.
//
//go:embed _static/example_readme.md
var ExampleReadmeContent string
