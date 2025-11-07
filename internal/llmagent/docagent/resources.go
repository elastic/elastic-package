package docagent

import _ "embed"

//go:embed _static/initial_prompt.txt
var InitialPrompt string

//go:embed _static/revision_prompt.txt
var RevisionPrompt string

//go:embed _static/limit_hit_prompt.txt
var LimitHitPrompt string
