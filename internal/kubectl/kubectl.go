// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package kubectl

import (
	"bytes"
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

func modifyKubernetesResources(action string, definitionPaths ...string) ([]byte, error) {
	args := []string{action}
	for _, definitionPath := range definitionPaths {
		args = append(args, "-f")
		args = append(args, definitionPath)
	}

	if action != "delete" { // "delete" supports only '-o name'
		args = append(args, "-o", "yaml")
	}

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

// applyKubernetesResourcesStdin applies a Kubernetes manifest provided as stdin.
// It returns the resources created as output and an error
func applyKubernetesResourcesStdin(input []byte) ([]byte, error) {
	// create kubectl apply command
	kubectlCmd := exec.Command("kubectl", "apply", "-f", "-", "-o", "yaml")
	//Stdin of kubectl command is the manifest provided
	kubectlCmd.Stdin = bytes.NewReader(input)
	errOutput := new(bytes.Buffer)
	kubectlCmd.Stderr = errOutput

	logger.Debugf("run command: %s", kubectlCmd)
	output, err := kubectlCmd.Output()
	if err != nil {
		return nil, errors.Wrapf(err, "kubectl apply failed (stderr=%q)", errOutput.String())
	}
	return output, nil
}
