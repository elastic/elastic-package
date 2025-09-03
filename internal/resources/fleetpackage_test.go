// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package resources

import (
	"errors"
	"fmt"
	"path/filepath"
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
	cases := []struct {
		title string
		name  string
	}{
		{title: "nginx", name: "nginx"},
		{title: "package not found", name: "sql_input"},
	}

	for _, c := range cases {
		t.Run(c.title, func(t *testing.T) {
			recordPath := filepath.Join("testdata", "kibana-8-mock-package-lifecycle-"+c.name)
			kibanaClient := kibanatest.NewClient(t, recordPath)
			if !assertPackageInstalled(t, kibanaClient, "not_installed", c.name) {
				t.FailNow()
			}

			fleetPackage := FleetPackage{
				RootPath: filepath.Join("..", "..", "test", "packages", "parallel", c.name),
			}
			manager := resource.NewManager()
			manager.RegisterProvider(DefaultKibanaProviderName, &KibanaProvider{Client: kibanaClient})
			_, err := manager.Apply(resource.Resources{&fleetPackage})
			assert.NoError(t, err)
			assertPackageInstalled(t, kibanaClient, "installed", c.name)

			fleetPackage.Absent = true
			_, err = manager.Apply(resource.Resources{&fleetPackage})
			assert.NoError(t, err)
			assertPackageInstalled(t, kibanaClient, "not_installed", c.name)
		})
	}
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

	p, err := client.GetPackage(t.Context(), packageName)
	var notFoundError *kibana.ErrPackageNotFound
	if errors.As(err, &notFoundError) {
		return assert.Equal(t, expected, "not_installed")
	} else if !assert.NoError(t, err) {
		return false
	}
	return assert.Equal(t, expected, p.Status)
}
