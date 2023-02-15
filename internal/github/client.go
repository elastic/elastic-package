// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package github

import (
	"net/http"

	"github.com/google/go-github/v32/github"
)

// UnauthorizedClient function returns unauthorized instance of Github API client.
func UnauthorizedClient() *github.Client {
	return github.NewClient(new(http.Client))
}
