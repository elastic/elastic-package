// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package system

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/elastic/elastic-package/internal/agentdeployer"
	"github.com/elastic/elastic-package/internal/kibana"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/servicedeployer"
	"github.com/elastic/elastic-package/internal/stack"
)

const (
	serviceStateFileName = "service.json"
	stateFolderName      = "state"
)

type ServiceState struct {
	OrigPolicy       kibana.Policy `json:"orig_policy"`
	EnrollPolicy     kibana.Policy `json:"enroll_policy"`
	CurrentPolicy    kibana.Policy `json:"current_policy"`
	Agent            kibana.Agent  `json:"agent"`
	ConfigFilePath   string        `json:"config_file_path"`
	VariantName      string        `json:"variant_name"`
	ServiceRunID     string        `json:"service_info_run_id"`
	AgentRunID       string        `json:"agent_info_run_id"`
	ServiceOutputDir string        `json:"service_output_dir"`
}

// stateFolderPath returns the folder where the state data is stored
func stateFolderPath(profilePath string) string {
	return filepath.Join(profilePath, stack.ProfileStackPath, stateFolderName)
}

func readServiceStateData(path string) (ServiceState, error) {
	var setupData ServiceState
	logger.Debugf("Reading test config from file %s", path)
	contents, err := os.ReadFile(path)
	if err != nil {
		return setupData, fmt.Errorf("failed to read test config %q: %w", path, err)
	}
	err = json.Unmarshal(contents, &setupData)
	if err != nil {
		return setupData, fmt.Errorf("failed to decode service options %q: %w", path, err)
	}
	return setupData, nil
}

type scenarioStateOpts struct {
	currentPolicy *kibana.Policy
	enrollPolicy  *kibana.Policy
	origPolicy    *kibana.Policy
	config        *testConfig
	agent         kibana.Agent
	agentInfo     agentdeployer.AgentInfo
	svcInfo       servicedeployer.ServiceInfo
}

func writeScenarioState(opts scenarioStateOpts, target string) error {
	data := ServiceState{
		OrigPolicy:       *opts.origPolicy,
		EnrollPolicy:     *opts.enrollPolicy,
		CurrentPolicy:    *opts.currentPolicy,
		Agent:            opts.agent,
		ConfigFilePath:   opts.config.Path,
		VariantName:      opts.config.ServiceVariantName,
		ServiceRunID:     opts.svcInfo.Test.RunID,
		AgentRunID:       opts.agentInfo.Test.RunID,
		ServiceOutputDir: opts.svcInfo.OutputDir,
	}
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshall service setup data: %w", err)
	}

	err = os.WriteFile(target, dataBytes, 0644)
	if err != nil {
		return fmt.Errorf("failed to write service setup JSON: %w", err)
	}
	return nil
}
