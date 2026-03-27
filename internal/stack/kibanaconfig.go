// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package stack

import (
	"fmt"
	"io"
	"os"

	"github.com/elastic/go-resource"

	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/profile"
)

// kibanaCustomContent returns a FileContent that appends the user's kibana.dev.yml
// (if present) to whatever was already written. The profile is retrieved from the
// resource context so this source can be declared statically.
func kibanaCustomContent() resource.FileContent {
	return func(ctx resource.Context, w io.Writer) error {
		var p *profile.Profile
		if ok := ctx.Provider("profile", &p); !ok {
			return fmt.Errorf("a profile is expected in the resource context")
		}

		customConfigPath := p.Path(KibanaDevConfigFile)
		customConfigData, err := os.ReadFile(customConfigPath)
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("failed to read custom kibana config: %w", err)
		}

		logger.Infof("Custom Kibana configuration detected at %s - this may affect Kibana behavior", customConfigPath)

		if _, err = w.Write([]byte("\n\n# Custom Kibana Configuration\n")); err != nil {
			return fmt.Errorf("failed to write custom config separator: %w", err)
		}

		if _, err = w.Write(customConfigData); err != nil {
			return fmt.Errorf("failed to write custom kibana config: %w", err)
		}

		return nil
	}
}
