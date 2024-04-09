// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package resources

import (
	"context"
	"fmt"
	"testing"

	"github.com/elastic/go-resource"
	"github.com/stretchr/testify/assert"

	"github.com/elastic/elastic-package/internal/kibana"
	kibanatest "github.com/elastic/elastic-package/internal/kibana/test"
)

func TestRequiredProvider(t *testing.T) {
	manager := resource.NewManager()
	_, err := manager.Apply(resource.Resources{
		&FleetPackage{
			RootPath: "../../test/packages/parallel/nginx",
		},
	})
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), fmt.Sprintf("provider %q must be explicitly defined", DefaultKibanaProviderName))
	}
}

func TestPackageLifecycle(t *testing.T) {
	kibanaClient := kibanatest.NewClient(t, "testdata/kibana-8-mock-package-lifecycle-nginx")
	if !assertPackageInstalled(t, kibanaClient, "not_installed", "nginx") {
		t.FailNow()
	}

	fleetPackage := FleetPackage{
		RootPath: "../../test/packages/parallel/nginx",
	}
	manager := resource.NewManager()
	manager.RegisterProvider(DefaultKibanaProviderName, &KibanaProvider{Client: kibanaClient})
	_, err := manager.Apply(resource.Resources{&fleetPackage})
	assert.NoError(t, err)
	assertPackageInstalled(t, kibanaClient, "installed", "nginx")

	fleetPackage.Absent = true
	_, err = manager.Apply(resource.Resources{&fleetPackage})
	assert.NoError(t, err)
	assertPackageInstalled(t, kibanaClient, "not_installed", "nginx")
}

func TestSystemPackageIsNotRemoved(t *testing.T) {
	kibanaClient := kibanatest.NewClient(t, "testdata/kibana-7-mock-system-package-is-not-removed")
	if !assertPackageInstalled(t, kibanaClient, "installed", "system") {
		t.FailNow()
	}

	fleetPackage := FleetPackage{
		RootPath: "../../test/packages/parallel/system",
		Absent:   true,
	}
	manager := resource.NewManager()
	manager.RegisterProvider(DefaultKibanaProviderName, &KibanaProvider{Client: kibanaClient})

	// Try to uninstall the package, it should not be installed.
	_, err := manager.Apply(resource.Resources{&fleetPackage})
	assert.NoError(t, err)
	assertPackageInstalled(t, kibanaClient, "installed", "system")

	// Try to force-uninstall the package, it should neither be uninstalled.
	fleetPackage.Force = true
	_, err = manager.Apply(resource.Resources{&fleetPackage})
	assert.NoError(t, err)
	assertPackageInstalled(t, kibanaClient, "installed", "system")
}

func assertPackageInstalled(t *testing.T, client *kibana.Client, expected string, packageName string) bool {
	t.Helper()

	p, err := client.GetPackage(context.Background(), packageName)
	if !assert.NoError(t, err) {
		return false
	}
	return assert.Equal(t, expected, p.Status)
}
