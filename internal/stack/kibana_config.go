// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package stack

import (
	"bytes"
	"fmt"
	"io"
	"os"

	"github.com/elastic/go-resource"

	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/profile"
)

// kibanaConfigWithCustomContent generates kibana.yml with custom config appended
func kibanaConfigWithCustomContent(profile *profile.Profile) func(resource.Context, io.Writer) error {
	return func(ctx resource.Context, w io.Writer) error {
		// First, generate the base kibana.yml from template
		var baseConfig bytes.Buffer
		baseTemplate := staticSource.Template("_static/kibana.yml.tmpl")
		err := baseTemplate(ctx, &baseConfig)
		if err != nil {
			return fmt.Errorf("failed to generate base kibana config: %w", err)
		}

		// Write base config to output
		_, err = w.Write(baseConfig.Bytes())
		if err != nil {
			return fmt.Errorf("failed to write base kibana config: %w", err)
		}

		// Check if custom config file exists
		customConfigPath := profile.Path(KibanaDevConfigFile)
		customConfigData, err := os.ReadFile(customConfigPath)
		if os.IsNotExist(err) {
			return nil // No custom config file, that's fine
		}
		if err != nil {
			return fmt.Errorf("failed to read custom kibana config: %w", err)
		}

		// Log warning that custom config is being applied
		logger.Warnf("Custom Kibana configuration detected at %s - this may affect Kibana behavior", customConfigPath)

		// Add separator comment
		_, err = w.Write([]byte("\n\n# Custom Kibana Configuration\n"))
		if err != nil {
			return fmt.Errorf("failed to write custom config separator: %w", err)
		}

		// Append raw custom config content without template processing
		_, err = w.Write(customConfigData)
		if err != nil {
			return fmt.Errorf("failed to write custom kibana config: %w", err)
		}

		return nil
	}
}
