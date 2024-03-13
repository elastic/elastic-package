// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package kubectl

import "github.com/elastic/elastic-package/internal/logger"

// Delete function removes resources from the Kubernetes cluster based on provided definitions.
func Delete(definitionsPath []string) error {
	_, err := modifyKubernetesResources("delete", definitionsPath)
	return err
}

// DeleteStdin function removes resources from the Kubernetes cluster based on provided definitions.
func DeleteStdin(out []byte) error {
	logger.Debugf("Delete Kubernetes stdin")
	_, err := deleteKubernetesResourcesStdin(out)
	return err
}
