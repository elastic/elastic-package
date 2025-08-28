// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package dump

import (
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/suite"

	kibanatest "github.com/elastic/elastic-package/internal/kibana/test"
	"github.com/elastic/elastic-package/internal/stack"
)

func TestDumpAgentPolicies(t *testing.T) {
	// Files for each suite are recorded automatically on first test run.
	// To add a new suite:
	// - Configure it here.
	// - Add the needed agent policies or add new integrations in new or existing agent policies in a running stack.
	// - Configure environment variables for this stack (eval "$(elastic-package stack shellinit)").
	// - Run tests.
	// - Check that recorded files make sense and commit them.
	// To update the suite:
	// - Reproduce the scenario as described in the comments.
	// - Remove the files that you want to update.
	// - Follow the same steps to create a new suite.
	// - Check if the changes are the expected ones and commit them.
	suites := []*agentPoliciesDumpSuite{
		&agentPoliciesDumpSuite{
			// To reproduce this scenario:
			// - Start stack with version 7.17.0.
			// - Install nginx package.
			AgentPolicy:        "499b5aa7-d214-5b5d-838b-3cd76469844e",
			PackageName:        "nginx",
			Record:             "./testdata/fleet-7-mock-dump-all",
			DumpDirAll:         "./testdata/fleet-7-dump/all",
			DumpDirPackage:     "./testdata/fleet-7-dump/package",
			DumpDirAgentPolicy: "./testdata/fleet-7-dump/agentpolicy",
		},
		&agentPoliciesDumpSuite{
			// To reproduce this scenario:
			// - Start stack with version 8.0.0.
			// - Install nginx package.
			AgentPolicy:        "fleet-server-policy",
			PackageName:        "nginx",
			Record:             "./testdata/fleet-8-mock-dump-all",
			DumpDirAll:         "./testdata/fleet-8-dump/all",
			DumpDirPackage:     "./testdata/fleet-8-dump/package",
			DumpDirAgentPolicy: "./testdata/fleet-8-dump/agentpolicy",
		},
	}

	for _, s := range suites {
		suite.Run(t, s)
	}
}

type agentPoliciesDumpSuite struct {
	suite.Suite

	// PackageName is the name of the package to filter agent policies.
	AgentPolicy string

	// AgentPolicy is the name of the agent policy to look for.
	PackageName string

	// Record is where responses from Kibana are recorded.
	Record string

	// DumpDirAll is where the expected dumped files are stored when looking for all agent policies.
	DumpDirAll string

	// DumpDirPackage is where the expected dumped files are stored when filtering by package the agent policies.
	DumpDirPackage string

	// DumpDirAgentPolicy is where the expected dumped files are stored when looking for a specific agent policy.
	DumpDirAgentPolicy string
}

func (s *agentPoliciesDumpSuite) SetupTest() {
	_, err := os.Stat(s.DumpDirAll)
	if errors.Is(err, os.ErrNotExist) {
		client, err := stack.NewKibanaClient()
		s.Require().NoError(err)

		dumper := NewAgentPoliciesDumper(client)
		n, err := dumper.DumpAll(s.T().Context(), s.DumpDirAll)
		s.Require().NoError(err)
		s.Require().Greater(n, 0)
	} else {
		s.Require().NoError(err)
	}

	_, err = os.Stat(s.DumpDirPackage)
	if errors.Is(err, os.ErrNotExist) {
		client, err := stack.NewKibanaClient()
		s.Require().NoError(err)

		dumper := NewAgentPoliciesDumper(client)
		n, err := dumper.DumpByPackage(s.T().Context(), s.DumpDirPackage, s.PackageName)
		s.Require().NoError(err)
		s.Require().Greater(n, 0)
	} else {
		s.Require().NoError(err)
	}

	_, err = os.Stat(s.DumpDirAgentPolicy)
	if errors.Is(err, os.ErrNotExist) {
		client, err := stack.NewKibanaClient()
		s.Require().NoError(err)

		dumper := NewAgentPoliciesDumper(client)
		err = dumper.DumpByName(s.T().Context(), s.DumpDirAgentPolicy, s.AgentPolicy)
		s.Require().NoError(err)
	} else {
		s.Require().NoError(err)
	}
}

func (s *agentPoliciesDumpSuite) TestDumpAll() {
	client := kibanatest.NewClient(s.T(), s.Record)

	outputDir := s.T().TempDir()
	dumper := NewAgentPoliciesDumper(client)
	n, err := dumper.DumpAll(s.T().Context(), outputDir)
	s.Require().NoError(err)

	filesExpected := countFiles(s.T(), s.DumpDirAll)
	s.Assert().Equal(filesExpected, n)

	filesFound := countFiles(s.T(), outputDir)
	s.Assert().Equal(filesExpected, filesFound)

	assertEqualDumps(s.T(), s.DumpDirAll, outputDir)
}

func (s *agentPoliciesDumpSuite) TestDumpByPackage() {
	client := kibanatest.NewClient(s.T(), s.Record)

	outputDir := s.T().TempDir()
	dumper := NewAgentPoliciesDumper(client)
	n, err := dumper.DumpByPackage(s.T().Context(), outputDir, s.PackageName)
	s.Require().NoError(err)

	filesExpected := countFiles(s.T(), s.DumpDirPackage)
	s.Assert().Equal(filesExpected, n)

	filesFound := countFiles(s.T(), outputDir)
	s.Assert().Equal(filesExpected, filesFound)

	assertEqualDumps(s.T(), s.DumpDirPackage, outputDir)
}

func (s *agentPoliciesDumpSuite) TestDumpByName() {
	client := kibanatest.NewClient(s.T(), s.Record)

	outputDir := s.T().TempDir()
	dumper := NewAgentPoliciesDumper(client)
	err := dumper.DumpByName(s.T().Context(), outputDir, s.AgentPolicy)
	s.Require().NoError(err)

	filesExpected := countFiles(s.T(), s.DumpDirAgentPolicy)
	s.Assert().Equal(filesExpected, 1)

	filesFound := countFiles(s.T(), outputDir)
	s.Assert().Equal(filesExpected, filesFound)

	assertEqualDumps(s.T(), s.DumpDirAgentPolicy, outputDir)
}
