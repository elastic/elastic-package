// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package kubectl

import (
	"context"
)

// Delete function removes resources from the Kubernetes cluster based on provided definitions.
func (k *Client) Delete(ctx context.Context, definitionsPath []string) error {
	_, err := k.modifyKubernetesResources(ctx, "delete", definitionsPath)
	return err
}

// DeleteStdin function removes resources from the Kubernetes cluster based on provided definitions.
func (k *Client) DeleteStdin(ctx context.Context, out []byte) error {
	k.logger.Debug("Delete Kubernetes stdin")
	_, err := k.deleteKubernetesResourcesStdin(ctx, out)
	return err
}
