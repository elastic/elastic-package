// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package system

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/elastic/elastic-package/internal/kibana"
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

func readConfigFileFromState(profilePath string) (string, error) {
	type stateData struct {
		ConfigFilePath string `json:"config_file_path"`
	}
	var serviceStateData stateData
	setupDataPath := filepath.Join(stateFolderPath(profilePath), serviceStateFileName)
	fmt.Printf("Reading service state data from file: %s\n", setupDataPath)
	contents, err := os.ReadFile(setupDataPath)
	if err != nil {
		return "", fmt.Errorf("failed to read service state data %q: %w", setupDataPath, err)
	}
	err = json.Unmarshal(contents, &serviceStateData)
	if err != nil {
		return "", fmt.Errorf("failed to decode service state data %q: %w", setupDataPath, err)
	}
	return serviceStateData.ConfigFilePath, nil
}

// stateFolderPath returns the folder where the state data is stored
func stateFolderPath(profilePath string) string {
	return filepath.Join(profilePath, stack.ProfileStackPath, stateFolderName)
}
