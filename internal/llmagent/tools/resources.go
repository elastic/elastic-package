package tools

import _ "embed"

// The embedded example_readme is an example of a high-quality integration readme, following the static template archetype,
// which will help the LLM follow an example.
//
//go:embed _static/example_readme.md
var ExampleReadmeContent string
