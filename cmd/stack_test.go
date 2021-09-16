// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateServicesFlag(t *testing.T) {
	t.Run("a non available service returns error", func(t *testing.T) {
		err := validateServicesFlag([]string{"non-existing-service"})
		require.Error(t, err)
	})

	t.Run("a non available service in a valid stack returns error", func(t *testing.T) {
		err := validateServicesFlag([]string{"elasticsearch", "non-existing-service"})
		require.Error(t, err)
	})

	t.Run("not possible to start a service twice", func(t *testing.T) {
		err := validateServicesFlag([]string{"elasticsearch", "elasticsearch"})
		require.Error(t, err)
	})

	availableStackServicesTest := []struct {
		services []string
	}{
		{services: []string{"elastic-agent"}},
		{services: []string{"elastic-agent", "elasticsearch"}},
		{services: []string{"elasticsearch"}},
		{services: []string{"kibana"}},
		{services: []string{"package-registry"}},
		{services: []string{"fleet-server"}},
		{services: []string{"elasticsearch", "fleet-server"}},
	}

	for _, srv := range availableStackServicesTest {
		t.Run(fmt.Sprintf("%v are available as a stack service", srv.services), func(t *testing.T) {
			err := validateServicesFlag(srv.services)
			require.Nil(t, err)
		})
	}

}
