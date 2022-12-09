// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package stack

import (
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/elastic/go-sysinfo"
	"github.com/elastic/go-sysinfo/types"
	"gotest.tools/v3/assert"
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

func Test_getParentInfo(t *testing.T) {
	ppid := os.Getppid()
	parent, err := sysinfo.Process(ppid)
	if err != nil {
		panic(err)
	}
	info, err := parent.Info()
	if err != nil {
		panic(err)
	}

	type args struct {
		ppid int
	}
	tests := []struct {
		name    string
		args    args
		want    types.ProcessInfo
		wantErr bool
	}{
		// TODO: Add test cases.
		{"test parent", args{ppid}, info, false},
		{"bogus ppid", args{999999}, types.ProcessInfo{}, true},
		{"no parent", args{1}, types.ProcessInfo{}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getParentInfo(tt.args.ppid)
			if (err != nil) != tt.wantErr {
				t.Errorf("getParentInfo() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getParentInfo() = %v, want %v", got, tt.want)
			}
		})
	}
}
