// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package servicedeployer

const (
	serviceLogsDirEnv = "SERVICE_LOGS_DIR"
	testRunIDEnv      = "TEST_RUN_ID"
)

// ServiceContext encapsulates context that is both available to a ServiceDeployer and
// populated by a DeployedService. The fields in ServiceContext may be used in handlebars
// templates in system test configuration files, for example: {{ Hostname }}.
type ServiceContext struct {
	// Name is the name of the service.
	Name string

	// Hostname is the host name of the service, as addressable from
	// the Agent container.
	Hostname string

	// Ports is a list of ports that the service listens on, as addressable
	// from the Agent container.
	Ports []int

	// Port points to the first port in the list of ports. It's provided as
	// a convenient shortcut as most services tend to listen on a single port.
	Port int

	// Logs contains folder paths for log files produced by the service.
	Logs struct {
		Folder struct {
			// Local contains the folder path where log files produced by
			// the service are stored on the local filesystem, i.e. where
			// elastic-package is running.
			Local string

			// Agent contains the folder path where log files produced by
			// the service are stored on the Agent container's filesystem.
			Agent string
		}
	}

	// Test related properties.
	Test struct {
		// RunID identifies the current test run.
		RunID string
	}

	// Agent related properties.
	Agent struct {
		// Host describes the machine which is running the agent.
		Host struct {
			// Name prefix for the host's name
			NamePrefix string
		}
	}

	// CustomProperties store additional data used to boot up the service, e.g. AWS credentials.
	CustomProperties map[string]interface{}
}

// Aliases method returned aliases to properties of the service context.
func (sc *ServiceContext) Aliases() map[string]interface{} {
	m := map[string]interface{}{
		serviceLogsDirEnv: func() interface{} {
			return sc.Logs.Folder.Agent
		},
		testRunIDEnv: func() interface{} {
			return sc.Test.RunID
		},
	}

	for k, v := range sc.CustomProperties {
		var that = v
		m[k] = func() interface{} { // wrap as function
			return that
		}
	}
	return m
}
