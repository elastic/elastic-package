// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package docker

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/logger"
)

// NetworkDescription describes the Docker network and connected Docker containers.
type NetworkDescription struct {
	Containers map[string]struct {
		Name string
	}
}

// ContainerDescription describes the Docker container.
type ContainerDescription struct {
	Config struct {
		Image  string
		Labels map[string]string
	}
	ID    string
	State struct {
		Status   string
		ExitCode int
		Health   *struct {
			Status string
			Log    []struct {
				Start    time.Time
				ExitCode int
				Output   string
			}
		}
	}
}

// String function dumps string representation of the container description.
func (c *ContainerDescription) String() string {
	b, err := json.Marshal(c)
	if err != nil {
		return "error: can't marshal container description"
	}
	return string(b)
}

// Pull downloads the latest available revision of the image.
func Pull(image string) error {
	cmd := exec.Command("docker", "pull", image)

	if logger.IsDebugMode() {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}

	logger.Debugf("run command: %s", cmd)
	err := cmd.Run()
	if err != nil {
		return errors.Wrap(err, "running docker command failed")
	}
	return nil
}

// ContainerID function returns the container ID for a given container name.
func ContainerID(containerName string) (string, error) {
	cmd := exec.Command("docker", "ps", "--filter", "name="+containerName, "--format", "{{.ID}}")
	errOutput := new(bytes.Buffer)
	cmd.Stderr = errOutput

	logger.Debugf("output command: %s", cmd)
	output, err := cmd.Output()
	if err != nil {
		return "", errors.Wrapf(err, "could not find \"%s\" container (stderr=%q)", containerName, errOutput.String())
	}
	containerIDs := strings.Fields(string(output))
	if len(containerIDs) != 1 {
		return "", fmt.Errorf("expected single %s container", containerName)
	}
	return containerIDs[0], nil
}

// ContainerIDsWithLabel function returns all the container IDs filtering per label.
func ContainerIDsWithLabel(key, value string) ([]string, error) {
	label := fmt.Sprintf("%s=%s", key, value)
	cmd := exec.Command("docker", "ps", "-a", "--filter", "label="+label, "--format", "{{.ID}}")
	errOutput := new(bytes.Buffer)
	cmd.Stderr = errOutput

	logger.Debugf("output command: %s", cmd)
	output, err := cmd.Output()
	if err != nil {
		return []string{}, errors.Wrapf(err, "error getting containers with label \"%s\" (stderr=%q)", label, errOutput.String())
	}
	containerIDs := strings.Fields(string(output))
	return containerIDs, nil
}

// InspectNetwork function returns the network description for the selected network.
func InspectNetwork(network string) ([]NetworkDescription, error) {
	cmd := exec.Command("docker", "network", "inspect", network)
	errOutput := new(bytes.Buffer)
	cmd.Stderr = errOutput

	logger.Debugf("output command: %s", cmd)
	output, err := cmd.Output()
	if err != nil {
		return nil, errors.Wrapf(err, "could not inspect the network (stderr=%q)", errOutput.String())
	}

	var networkDescriptions []NetworkDescription
	err = json.Unmarshal(output, &networkDescriptions)
	if err != nil {
		return nil, errors.Wrapf(err, "can't unmarshal network inspect for %s (stderr=%q)", network, errOutput.String())
	}
	return networkDescriptions, nil
}

// ConnectToNetwork function connects the container to the selected Docker network.
func ConnectToNetwork(containerID, network string) error {
	cmd := exec.Command("docker", "network", "connect", network, containerID)
	errOutput := new(bytes.Buffer)
	cmd.Stderr = errOutput

	logger.Debugf("run command: %s", cmd)
	if err := cmd.Run(); err != nil {
		return errors.Wrapf(err, "could not attach container to the stack network (stderr=%q)", errOutput.String())
	}
	return nil
}

// InspectContainers function inspects selected Docker containers.
func InspectContainers(containerIDs ...string) ([]ContainerDescription, error) {
	args := []string{"inspect"}
	args = append(args, containerIDs...)
	cmd := exec.Command("docker", args...)

	errOutput := new(bytes.Buffer)
	cmd.Stderr = errOutput

	logger.Debugf("output command: %s", cmd)
	output, err := cmd.Output()
	if err != nil {
		return nil, errors.Wrapf(err, "could not inspect containers (stderr=%q)", errOutput.String())
	}

	var containerDescriptions []ContainerDescription
	err = json.Unmarshal(output, &containerDescriptions)
	if err != nil {
		return nil, errors.Wrapf(err, "can't unmarshal container inspect for %s (stderr=%q)", strings.Join(containerIDs, ","), errOutput.String())
	}
	return containerDescriptions, nil
}

// Copy function copies resources from the container to the local destination.
func Copy(containerName, containerPath, localPath string) error {
	cmd := exec.Command("docker", "cp", containerName+":"+containerPath, localPath)
	errOutput := new(bytes.Buffer)
	cmd.Stderr = errOutput

	logger.Debugf("run command: %s", cmd)
	if err := cmd.Run(); err != nil {
		return errors.Wrapf(err, "could not copy files from the container (stderr=%q)", errOutput.String())
	}
	return nil
}
