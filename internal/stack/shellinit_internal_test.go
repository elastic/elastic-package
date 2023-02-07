// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package stack

import (
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
		{"bash code template", args{"bash"}, posixTemplate},
		{"fish code template", args{"fish"}, fishTemplate},
		{"sh code template", args{"sh"}, posixTemplate},
		{"zsh code template", args{"zsh"}, posixTemplate},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, _ := initTemplate(tt.args.s); got != tt.want {
				t.Errorf("CodeTemplate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCodeTemplate_wrongInput(t *testing.T) {
	_, err := initTemplate("invalid shell type")
	assert.Error(t, err, "shell type is unknown, should be one of "+strings.Join(availableShellTypes, ", "))
}

func Test_getShellName(t *testing.T) {
	type args struct {
		exe string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{"linux exec", args{exe: "bash"}, "bash"},
		{"windows exec", args{exe: "cmd.exe"}, "cmd"},
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
