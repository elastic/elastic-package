// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"fmt"
	"os"
	"reflect"
	"testing"

	"github.com/elastic/go-sysinfo"
	"github.com/elastic/go-sysinfo/types"
	"github.com/stretchr/testify/require"
)

func TestValidateServicesFlag(t *testing.T) {
	t.Run("a non available service returns error", func(t *testing.T) {
		err := validateServicesFlag([]string{"non-existing-service"})
		require.Error(t, err)
	})

	t.Run("a non available service in a valid stack returns error", func(t *testing.T) {
		err := validateServicesFlag([]string{"elasticsearch", "non-existing-service"})
		require.Error(t, err)
	})

	t.Run("not possible to start a service twice", func(t *testing.T) {
		err := validateServicesFlag([]string{"elasticsearch", "elasticsearch"})
		require.Error(t, err)
	})

	availableStackServicesTest := []struct {
		services []string
	}{
		{services: []string{"elastic-agent"}},
		{services: []string{"elastic-agent", "elasticsearch"}},
		{services: []string{"elasticsearch"}},
		{services: []string{"kibana"}},
		{services: []string{"package-registry"}},
		{services: []string{"fleet-server"}},
		{services: []string{"elasticsearch", "fleet-server"}},
	}

	for _, srv := range availableStackServicesTest {
		t.Run(fmt.Sprintf("%v are available as a stack service", srv.services), func(t *testing.T) {
			err := validateServicesFlag(srv.services)
			require.Nil(t, err)
		})
	}

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
