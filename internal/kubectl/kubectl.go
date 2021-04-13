// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package kubectl

import (
	"bytes"
	"fmt"
	"os/exec"

	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/logger"
)

// CurrentContext function returns the selected Kubernetes context.
func CurrentContext() (string, error) {
	cmd := exec.Command("kubectl", "config", "current-context")
	errOutput := new(bytes.Buffer)
	cmd.Stderr = errOutput

	logger.Debugf("output command: %s", cmd)
	output, err := cmd.Output()
	if err != nil {
		return "", errors.Wrapf(err, "kubectl command failed (stderr=%q)", errOutput.String())
	}
	return string(bytes.TrimSpace(output)), nil
}

// Apply function adds resources to the Kubernetes cluster based on provided definitions.
func Apply(definitionPaths ...string) error {
	logger.Debugf("Apply Kubernetes definitions")
	out, err := modifyKubernetesResources("apply", definitionPaths...)
	if err != nil {
		return errors.Wrap(err, "can't modify Kubernetes resources (apply)")
	}

	logger.Debugf("Unmarshal \"apply\" command output")
	output, err := unmarshalApplyCommandOutput(out)
	if err != nil {
		return errors.Wrap(err, "can't unmarshal \"apply\" output")
	}
	fmt.Println(output)

	return nil
}

// Delete function removes resources from the Kubernetes cluster based on provided definitions.
func Delete(definitionPaths ...string) error {
	_, err := modifyKubernetesResources("delete", definitionPaths...)
	return err
}

func modifyKubernetesResources(action string, definitionPaths ...string) ([]byte, error) {
	args := []string{action}
	for _, definitionPath := range definitionPaths {
		args = append(args, "-f")
		args = append(args, definitionPath)
	}
	args = append(args, "-o", "yaml")

	cmd := exec.Command("kubectl", args...)
	errOutput := new(bytes.Buffer)
	cmd.Stderr = errOutput

	logger.Debugf("run command: %s", cmd)
	output, err := cmd.Output()
	if err != nil {
		return nil, errors.Wrapf(err, "kubectl apply failed (stderr=%q)", errOutput.String())
	}
	return output, nil
}

func unmarshalApplyCommandOutput(output []byte) (*resource, error) {

}