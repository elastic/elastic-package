// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package docs

import (
	"embed"
)

// Input definitions

//go:embed _static/inputs/*.yml
var InputDescriptions embed.FS

// Input Agent templates

//go:embed _static/agent/*.yml.hbs
var AgentTemplates embed.FS
