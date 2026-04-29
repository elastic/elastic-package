// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

// Package gorules contains custom go-ruleguard rules consumed by gocritic.
//
// These rules are not compiled as part of the program; they are loaded by
// the ruleguard checker. The build tag prevents `go build ./...` from
// trying to compile them (the dsl import is not in go.mod).
//
//go:build ruleguard

package gorules

import "github.com/quasilyte/go-ruleguard/dsl"

// errSwallowedReturnNil flags `if err != nil { return nil }` patterns where
// the checked error value is silently discarded. Returning nil after an
// error check is almost always a bug; if intentional, suppress with
// //nolint:gocritic and a comment explaining why.
func errSwallowedReturnNil(m dsl.Matcher) {
	m.Match(`if $err != nil { return nil }`).
		Where(m["err"].Type.Is("error")).
		Report("error checked but discarded; return $err (or wrap it)")
}
