// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package agentdeployer

const (
	localCACertEnv      = "LOCAL_CA_CERT"
	serviceLogsDirEnv   = "SERVICE_LOGS_DIR"
	testRunIDEnv        = "TEST_RUN_ID"
	fleetPolicyEnv      = "FLEET_TOKEN_POLICY_NAME"
	agentHostnameEnv    = "AGENT_HOSTNAME"
	elasticAgentTagsEnv = "ELASTIC_AGENT_TAGS"
)

// AgentInfo encapsulates context that is both available to a AgentDeployer and
// populated by a DeployedAgent. The fields in AgentInfo may be used in handlebars
// templates in system test configuration files, for example: {{ Hostname }}.
type AgentInfo struct {
	// Name is the name of the service.
	Name string

	// Hostname is the host name of the service, as addressable from
	// the Agent container.
	Hostname string

	// NetworkName is the name of the docker network created for the agent,
	// required to connect the Service with the agent.
	NetworkName string

	// Agent Policy related properties
	Policy struct {
		// Name is the name of the test Agent Policy created for the given agent
		Name string
		// ID is the name of the test Agent Policy created for the given agent
		ID string
	}

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

		// User user to run Elastic Agent process
		User string
		// PidMode selects the host PID mode
		// (From docker-compose docs) Turns on sharing between container and the host
		// operating system the PID address space
		PidMode string
		// Runtime is the selected runtime to run the Elastic Agent process
		Runtime string
		// LinuxCapabilities is a list of the capabilities needed to run the Elastic Agent process
		LinuxCapabilities []string
	}

	// CustomProperties store additional data used to boot up the service, e.g. AWS credentials.
	CustomProperties map[string]interface{}

	// Directory to store agent configuration files
	ConfigDir string
}
