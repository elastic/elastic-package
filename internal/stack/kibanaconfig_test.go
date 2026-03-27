// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package stack

import (
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/elastic/go-resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/elastic-package/internal/profile"
)

func createTestProfile(t *testing.T) *profile.Profile {
	t.Helper()
	profilesPath := t.TempDir()
	err := profile.CreateProfile(profile.Options{
		ProfilesDirPath: profilesPath,
		Name:            "test",
	})
	require.NoError(t, err)

	p, err := profile.LoadProfileFrom(profilesPath, "test")
	require.NoError(t, err)
	return p
}

func profileContext(p *profile.Profile) resource.Context {
	m := resource.NewManager()
	m.RegisterProvider("profile", p)
	return m.Context(context.Background())
}

func TestKibanaCustomContent(t *testing.T) {
	cases := []struct {
		name          string
		devConfigFile string
		wantOutput    string
		wantErr       string
	}{
		{
			name:       "no custom config file",
			wantOutput: "",
		},
		{
			name:          "custom config appended with separator",
			devConfigFile: "logging.loggers:\n  - name: root\n    level: debug\n",
			wantOutput:    "\n\n# Custom Kibana Configuration\nlogging.loggers:\n  - name: root\n    level: debug\n",
		},
		{
			name:          "custom config content not template-processed",
			devConfigFile: "server.name: kibana-{{ fact \"kibana_version\" }}\n",
			wantOutput:    "\n\n# Custom Kibana Configuration\nserver.name: kibana-{{ fact \"kibana_version\" }}\n",
		},
		{
			name:    "error reading custom config",
			wantErr: "failed to read custom kibana config",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := createTestProfile(t)
			customConfigPath := p.Path(KibanaDevConfigFile)

			if tc.wantErr != "" {
				// Make the path a directory so ReadFile fails.
				err := os.MkdirAll(customConfigPath, 0755)
				require.NoError(t, err)
			} else if tc.devConfigFile != "" {
				err := os.WriteFile(customConfigPath, []byte(tc.devConfigFile), 0644)
				require.NoError(t, err)
			}

			ctx := profileContext(p)
			var buf bytes.Buffer
			err := kibanaCustomContent()(ctx, &buf)

			if tc.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.wantOutput, buf.String())
			}
		})
	}
}
