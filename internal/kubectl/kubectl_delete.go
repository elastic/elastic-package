// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package kubectl

import "context"

// Delete function removes resources from the Kubernetes cluster based on provided definitions.
func Delete(ctx context.Context, definitionsPath []string) error {
	_, err := modifyKubernetesResources(ctx, "delete", definitionsPath)
	return err
}
