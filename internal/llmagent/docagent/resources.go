// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package docagent

import _ "embed"

//go:embed _static/initial_prompt.txt
var InitialPrompt string

//go:embed _static/revision_prompt.txt
var RevisionPrompt string

//go:embed _static/limit_hit_prompt.txt
var LimitHitPrompt string
