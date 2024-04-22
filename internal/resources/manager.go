// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

// This file contains helpers so we don't need to import go-resource out of this package.

package resources

import "github.com/elastic/go-resource"

type Resources = resource.Resources

type Manager = resource.Manager

func NewManager() *Manager {
	return resource.NewManager()
}
