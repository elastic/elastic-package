// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package stack

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCodeTemplate(t *testing.T) {
	type args struct {
		s string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{"bash code template", args{"bash"}, posixPattern},
		{"fish code template", args{"fish"}, fishPattern},
		{"sh code template", args{"sh"}, posixPattern},
		{"zsh code template", args{"zsh"}, posixPattern},
		{"pwsh code template", args{"pwsh"}, powershellPattern},
		{"powershell code template", args{"powershell"}, powershellPattern},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, _ := selectPattern(tt.args.s); got != tt.want {
				t.Errorf("CodeTemplate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestShellInit(t *testing.T) {
	config := InitConfig{
		ElasticsearchHostPort: "https://elastic.example.com:9200",
		ElasticsearchUsername: "admin",
		ElasticsearchPassword: "secret",
		KibanaHostPort:        "https://kibana.example.com:5601",
	}

	expected := strings.TrimSpace(`
export ELASTIC_PACKAGE_ELASTICSEARCH_API_KEY=
export ELASTIC_PACKAGE_ELASTICSEARCH_HOST=https://elastic.example.com:9200
export ELASTIC_PACKAGE_ELASTICSEARCH_USERNAME=admin
export ELASTIC_PACKAGE_ELASTICSEARCH_PASSWORD=secret
export ELASTIC_PACKAGE_KIBANA_HOST=https://kibana.example.com:5601
export ELASTIC_PACKAGE_CA_CERT=
`)

	result, err := shellInitWithConfig(&config, "bash")
	require.NoError(t, err)

	assert.Equal(t, expected, result)
}

func TestCodeTemplate_wrongInput(t *testing.T) {
	_, err := selectPattern("invalid shell type")
	assert.Error(t, err, "shell type is unknown, should be one of "+strings.Join(availableShellTypes, ", "))
}

func Test_getShellName(t *testing.T) {
	type args struct {
		exe string
	}
	type testCase struct {
		name string
		args args
		want string
	}
	var tests []testCase
	if filepath.Separator == '\\' {
		tests = []testCase{
			{"windows relative path", args{exe: `cmd.exe`}, "cmd"}, // Not sure if this case is real.
			{"windows exec", args{exe: `C:\Windows\System32\cmd.exe`}, "cmd"},
		}
	} else {
		tests = []testCase{
			{"linux exec", args{exe: "/usr/bin/bash"}, "bash"},
			{"linux upgraded exec", args{exe: "/usr/bin/bash (deleted)"}, "bash"},
			{"windows relative path", args{exe: `cmd.exe`}, "cmd"}, // Not sure if this case is real.
			{"windows exec", args{exe: `C:/Windows/System32/cmd.exe`}, "cmd"},
		}
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getShellName(tt.args.exe); got != tt.want {
				t.Errorf("getShellName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetParentShell(t *testing.T) {
	shell, err := getParentShell()
	require.NoError(t, err)
	assert.NotEmpty(t, shell)
}
