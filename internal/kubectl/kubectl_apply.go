// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package kubectl

import (
	"context"
	"fmt"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
	"helm.sh/helm/v3/pkg/kube"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	kresource "k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/rest"

	"github.com/elastic/elastic-package/internal/logger"
)

const readinessTimeout = 10 * time.Minute

type resource struct {
	APIVersion string   `yaml:"apiVersion"`
	Kind       string   `yaml:"kind"`
	Metadata   metadata `yaml:"metadata"`
	Status     *status  `yaml:"status"`

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
	Conditions *[]condition
}

//lint:ignore U1000 unused, but let's keep it by now.
func (s status) isReady() (*condition, bool) {
	if s.Conditions == nil {
		return nil, false // safe fallback
	}

	for _, c := range *s.Conditions {
		if (c.Type == "Ready" || c.Type == "Available") && !strings.Contains(c.Message, "does not have minimum availability") {
			return &c, true
		}
	}
	return nil, false
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
func Apply(ctx context.Context, definitionsPath []string) error {
	logger.Debugf("Apply Kubernetes custom definitions")
	out, err := modifyKubernetesResources(ctx, "apply", definitionsPath)
	if err != nil {
		return fmt.Errorf("can't modify Kubernetes resources (apply): %w", err)
	}

	logger.Debugf("Handle \"apply\" command output")
	err = handleApplyCommandOutput(out)
	if err != nil {
		return fmt.Errorf("can't handle command output: %w", err)
	}
	return nil
}

// ApplyStdin function adds resources to the Kubernetes cluster based on provided stdin.
func ApplyStdin(ctx context.Context, input []byte) error {
	logger.Debugf("Apply Kubernetes stdin")
	out, err := applyKubernetesResourcesStdin(ctx, input)
	if err != nil {
		return fmt.Errorf("can't modify Kubernetes resources (apply stdin): %w", err)
	}

	logger.Debugf("Handle \"apply\" command output")
	err = handleApplyCommandOutput(out)
	if err != nil {
		return fmt.Errorf("can't handle command output: %w", err)
	}
	return nil
}

func handleApplyCommandOutput(out []byte) error {
	logger.Debugf("Extract resources from command output")
	resources, err := extractResources(out)
	if err != nil {
		return fmt.Errorf("can't extract resources: %w", err)
	}

	logger.Debugf("Wait for ready resources")
	err = waitForReadyResources(resources)
	if err != nil {
		return fmt.Errorf("resources are not ready: %w", err)
	}
	return nil
}

func waitForReadyResources(resources []resource) error {
	var resList kube.ResourceList
	for _, r := range resources {
		resInfo, err := createResourceInfo(r)
		if err != nil {
			return fmt.Errorf("can't fetch resource info: %w", err)
		}
		resList = append(resList, resInfo)
	}

	kubeClient := kube.New(nil)
	kubeClient.Log = func(s string, i ...interface{}) {
		logger.Debugf(s, i...)
	}
	// In case of elastic-agent daemonset Wait will not work as expected
	// because in single node clusters one pod of the daemonset can always
	// be unavailable (DaemonSet.spec.updateStrategy.rollingUpdate.maxUnavailable defaults to 1).
	// daemonSetReady will return true regardless of the pod not being ready yet.
	// Can be solved with multi-node clusters.
	// TODO: Support context cancelation in this wait. We rely on a helm waiter
	// that doesn't support it.
	err := kubeClient.Wait(resList, readinessTimeout)
	if err != nil {
		return fmt.Errorf("waiter failed: %w", err)
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
		return nil, fmt.Errorf("can't unmarshal command output: %w", err)
	}
	return &r, nil
}

func createResourceInfo(r resource) (*kresource.Info, error) {
	scope := meta.RESTScopeNamespace
	if r.Metadata.Namespace == "" {
		scope = meta.RESTScopeRoot
	}

	restClient, err := createRESTClientForResource(r)
	if err != nil {
		return nil, fmt.Errorf("can't create REST client for resource: %w", err)
	}

	var group string
	var version string

	if !strings.Contains(r.APIVersion, "/") {
		version = r.APIVersion
	} else {
		i := strings.Index(r.APIVersion, "/")
		group = r.APIVersion[:i]
		version = r.APIVersion[i+1:]
	}

	resInfo := &kresource.Info{
		Name:      r.Metadata.Name,
		Namespace: r.Metadata.Namespace,
		Mapping: &meta.RESTMapping{
			GroupVersionKind: schema.GroupVersionKind{
				Group:   group,
				Version: version,
				Kind:    strings.ToLower(r.Kind),
			},
			Resource: schema.GroupVersionResource{
				Group:    group,
				Version:  version,
				Resource: r.Kind + "s"}, // "s" is for plural
			Scope: scope,
		},
		Client: restClient,
	}

	logger.Debugf("Sync resource info: %s (kind: %s, namespace: %s)", r.Metadata.Name, r.Kind, r.Metadata.Namespace)
	err = resInfo.Get()
	if err != nil {
		return nil, fmt.Errorf("can't sync resource info: %w", err)
	}
	return resInfo, nil
}

func createRESTClientForResource(r resource) (*rest.RESTClient, error) {
	restClientGetter := genericclioptions.NewConfigFlags(true)
	restConfig, err := restClientGetter.ToRESTConfig()
	if err != nil {
		return nil, fmt.Errorf("can't convert to REST config: %w", err)
	}
	restConfig.NegotiatedSerializer = kresource.UnstructuredPlusDefaultContentConfig().NegotiatedSerializer

	if !strings.Contains(r.APIVersion, "/") {
		restConfig.APIPath = "/api/" + r.APIVersion
	} else {
		restConfig.APIPath = "/apis/" + r.APIVersion
	}

	restClient, err := rest.UnversionedRESTClientFor(restConfig)
	if err != nil {
		return nil, fmt.Errorf("can't create unversioned REST client: %w", err)
	}
	return restClient, nil
}
