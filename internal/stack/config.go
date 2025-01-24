// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package stack

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/elastic/elastic-package/internal/profile"
)

var configFileName = "config.json"

type Config struct {
	Provider   string            `json:"provider,omitempty"`
	Parameters map[string]string `json:"parameters,omitempty"`

	ElasticsearchAPIKey   string `json:"elasticsearch_api_key,omitempty"`
	ElasticsearchHost     string `json:"elasticsearch_host,omitempty"`
	ElasticsearchUsername string `json:"elasticsearch_username,omitempty"`
	ElasticsearchPassword string `json:"elasticsearch_password,omitempty"`
	KibanaHost            string `json:"kibana_host,omitempty"`
	CACertFile            string `json:"ca_cert_file,omitempty"`

	OutputID      string `json:"output_id,omitempty"`
	FleetServerID string `json:"fleet_server_id,omitempty"`

	// EnrollmentToken is the token used during initialization, it can expire,
	// so don't persist it, it won't be reused.
	EnrollmentToken string `json:"-"`

	// FleetServiceToken is the service token used during initialization when
	// a local Fleet Server is needed.
	FleetServiceToken string `json:"-"`
}

func configPath(profile *profile.Profile) string {
	return profile.Path(ProfileStackPath, configFileName)
}

func defaultConfig() Config {
	return Config{
		Provider: DefaultProvider,

		// Hard-coded default values for backwards-compatibility.
		ElasticsearchHost:     "https://127.0.0.1:9200",
		ElasticsearchUsername: elasticsearchUsername,
		ElasticsearchPassword: elasticsearchPassword,
		KibanaHost:            "https://127.0.0.1:5601",
	}
}

func LoadConfig(profile *profile.Profile) (Config, error) {
	d, err := os.ReadFile(configPath(profile))
	if errors.Is(err, os.ErrNotExist) {
		config := defaultConfig()
		caCertFile := profile.Path(CACertificateFile)
		// Use CA file in the profile only if it exists.
		if _, err := os.Stat(caCertFile); err == nil {
			config.CACertFile = caCertFile
		}
		return config, nil
	}
	if err != nil {
		return Config{}, fmt.Errorf("failed to read stack config: %w", err)
	}

	var config Config
	err = json.Unmarshal(d, &config)
	if err != nil {
		return Config{}, fmt.Errorf("failed to decode stack config: %w", err)
	}

	return config, nil
}

func storeConfig(profile *profile.Profile, config Config) error {
	d, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to encode stack config: %w", err)
	}

	err = os.MkdirAll(filepath.Dir(configPath(profile)), 0755)
	if err != nil {
		return fmt.Errorf("failed to create parent directory: %w", err)
	}

	err = os.WriteFile(configPath(profile), d, 0644)
	if err != nil {
		return fmt.Errorf("failed to write stack config: %s", err)
	}

	return nil
}

func printUserConfig(printer Printer, config Config) {
	if printer == nil {
		return
	}
	printer.Printf("Elasticsearch host: %s\n", config.ElasticsearchHost)
	printer.Printf("Kibana host: %s\n", config.KibanaHost)
	printer.Printf("Username: %s\n", config.ElasticsearchUsername)
	printer.Printf("Password: %s\n", config.ElasticsearchPassword)
}
