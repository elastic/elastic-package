package kubectl

import (
	"fmt"
	"time"

	"github.com/elastic/elastic-package/internal/logger"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
)

type resource struct {
	Kind     string   `yaml:"kind"`
	Metadata metadata `yaml:"metadata"`
	Status   *status  `yaml:"status"`

	Items []resource `yaml:"items"`
}

func (r resource) String() string {
	return fmt.Sprintf("%s (kind: %s, namespace: %s)", r.Metadata.Name, r.Kind, r.Metadata.Namespace)
}

type metadata struct {
	Name      string `yaml:"name"`
	Namespace string `yaml:"namespace"`
}

type status struct {
	Conditions []condition
}

func (s status) lastCondition() *condition {
	panic("TODO")
}

type condition struct {
	LastUpdateTime time.Time `yaml:"lastUpdateTime"`
	Message        string    `yaml:"message"`
	Type           string    `yaml:"type"`
}

func (c condition) String() string {
	return fmt.Sprintf("%s (type: %s, time: %v)", c.Message, c.Type, c.LastUpdateTime)
}

// Apply function adds resources to the Kubernetes cluster based on provided definitions.
func Apply(definitionPaths ...string) error {
	logger.Debugf("Apply Kubernetes definitions")
	out, err := modifyKubernetesResources("apply", definitionPaths...)
	if err != nil {
		return errors.Wrap(err, "can't modify Kubernetes resources (apply)")
	}

	logger.Debugf("Handle \"apply\" command output")
	err = handleApplyCommandOutput(out)
	if err != nil {
		return errors.Wrap(err, "can't handle command output")
	}
	return nil
}

func handleApplyCommandOutput(out []byte) error {
	logger.Debugf("Extract resources from command output")
	resources, err := extractResources(out)
	if err != nil {
		return errors.Wrap(err, "can't extract resources")
	}

	logger.Debugf("Wait for ready resources")
	err = waitForReadyResources(resources)
	if err != nil {
		return errors.Wrap(err, "resources are not ready")
	}
	return nil
}

func waitForReadyResources(resources []resource) error {
	for _, r := range resources {
		logger.Debugf("Wait for resource: %s", r.String())

		for {
			out, err := getKubernetesResource(r.Kind, r.Metadata.Name, r.Metadata.Namespace)
			if err != nil {
				return errors.Wrap(err, "can't get Kubernetes resource")
			}

			res, err := extractResource(out)
			if err != nil {
				return errors.Wrap(err, "can't extract Kubernetes resource")
			}

			if res.Status == nil {
				logger.Debugf("The resource doesn't define status conditions. Skipping verification.")
				break
			}

			last := res.Status.lastCondition()
			if last == nil {
				logger.Debugf("No status condition available yet.")
				goto wait
			}

			logger.Debugf("Status condition: %s", last.String())
			if last.Type == "Ready" || last.Type == "Available" {
				break
			}

		wait:
			time.Sleep(time.Second)
		}
	}
	return nil
}

func extractResources(output []byte) ([]resource, error) {
	r, err := extractResource(output)
	if err != nil {
		return nil, err
	}

	if len(r.Items) == 0 {
		return []resource{*r}, nil
	}
	return r.Items, nil
}

func extractResource(output []byte) (*resource, error) {
	var r resource
	err := yaml.Unmarshal(output, &r)
	if err != nil {
		return nil, errors.Wrap(err, "can't unmarshal command output")
	}
	return &r, nil
}
