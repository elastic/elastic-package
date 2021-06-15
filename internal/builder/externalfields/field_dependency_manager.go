// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package externalfields

type fieldDependencyManager struct{}

func createFieldDependencyManager(dep dependencies) (*fieldDependencyManager, error) {
	return &fieldDependencyManager{}, nil
}

func (fdm *fieldDependencyManager) resolveFile(content []byte) ([]byte, bool, error) {
	panic("not implemented")
}
