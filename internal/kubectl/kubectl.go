// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package kubectl

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"path/filepath"

	"github.com/elastic/elastic-package/internal/logger"
)

const kustomizationFile = "kustomization.yaml"

type client struct {
	logger *slog.Logger
}

type KubectlOption func(k *client)

func NewKubectlClient(opts ...KubectlOption) *client {
	c := client{
		logger: logger.Logger,
	}

	for _, opt := range opts {
		opt(&c)
	}
	return &c
}

func WithLogger(log *slog.Logger) KubectlOption {
	return func(k *client) {
		k.logger = log
	}
}

// CurrentContext function returns the selected Kubernetes context.
func (k *client) CurrentContext(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "kubectl", "config", "current-context")
	errOutput := new(bytes.Buffer)
	cmd.Stderr = errOutput

	k.logger.Debug("output command", slog.String("command", cmd.String()))
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("kubectl command failed (stderr=%q): %w", errOutput.String(), err)
	}
	return string(bytes.TrimSpace(output)), nil
}

func (k *client) modifyKubernetesResources(ctx context.Context, action string, definitionPaths []string) ([]byte, error) {
	args := []string{action}
	for _, definitionPath := range definitionPaths {
		if filepath.Base(definitionPath) == kustomizationFile {
			args = []string{action, "-k", filepath.Dir(definitionPath)}
			break
		}
		args = append(args, "-f")
		args = append(args, definitionPath)
	}

	if action != "delete" { // "delete" supports only '-o name'
		args = append(args, "-o", "yaml")
	}

	cmd := exec.CommandContext(ctx, "kubectl", args...)
	errOutput := new(bytes.Buffer)
	cmd.Stderr = errOutput

	k.logger.Debug("output command", slog.String("command", cmd.String()))
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("kubectl apply failed (stderr=%q): %w", errOutput.String(), err)
	}
	return output, nil
}

// applyKubernetesResourcesStdin applies a Kubernetes manifest provided as stdin.
// It returns the resources created as output and an error
func (k *client) applyKubernetesResourcesStdin(ctx context.Context, input []byte) ([]byte, error) {
	// create kubectl apply command
	kubectlCmd := exec.CommandContext(ctx, "kubectl", "apply", "-f", "-", "-o", "yaml")
	//Stdin of kubectl command is the manifest provided
	kubectlCmd.Stdin = bytes.NewReader(input)
	errOutput := new(bytes.Buffer)
	kubectlCmd.Stderr = errOutput

	k.logger.Debug("run command", slog.String("cmd", kubectlCmd.String()))
	output, err := kubectlCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("kubectl apply failed (stderr=%q): %w", errOutput.String(), err)
	}
	return output, nil
}

// deleteKubernetesResourcesStdin deletes a Kubernetes manifest provided as stdin.
// It returns the resources deleted as output and an error
func (k *client) deleteKubernetesResourcesStdin(ctx context.Context, input []byte) ([]byte, error) {
	// create kubectl apply command
	kubectlCmd := exec.CommandContext(ctx, "kubectl", "delete", "-f", "-")
	// Stdin of kubectl command is the manifest provided
	kubectlCmd.Stdin = bytes.NewReader(input)
	errOutput := new(bytes.Buffer)
	kubectlCmd.Stderr = errOutput

	k.logger.Debug("run command", slog.String("command", kubectlCmd.String()))
	output, err := kubectlCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("kubectl delete failed (stderr=%q): %w", errOutput.String(), err)
	}
	return output, nil
}
