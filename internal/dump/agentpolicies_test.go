// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package dump

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/elastic/elastic-package/internal/kibana"
)

func TestDumpAgentPolicies(t *testing.T) {
	// Files for each suite are recorded automatically on first test run.
	// To add a new suite:
	// - Configure it here.
	// - Add the needed agent policies or add new integrations in new or existing agent policies in a running stack.
	// - Configure environment variables for this stack (eval "$(elastic-package stack shellinit)").
	// - Run tests.
	// - Check that recorded files make sense and commit them.
	suites := []*agentPoliciesDumpSuite{
		&agentPoliciesDumpSuite{
			AgentPolicy:        "499b5aa7-d214-5b5d-838b-3cd76469844e",
			PackageName:        "nginx",
			RecordDir:          "./testdata/fleet-7-mock-dump-all",
			DumpDirAll:         "./testdata/fleet-7-dump/all",
			DumpDirPackage:     "./testdata/fleet-7-dump/package",
			DumpDirAgentPolicy: "./testdata/fleet-7-dump/agentpolicy",
		},
		&agentPoliciesDumpSuite{
			AgentPolicy:        "fleet-server-policy",
			PackageName:        "nginx",
			RecordDir:          "./testdata/fleet-8-mock-dump-all",
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

	// RecordDir is where responses from Kibana are recorded.
	RecordDir string

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
		client, err := kibana.NewClient()
		s.Require().NoError(err)

		dumper := NewAgentPoliciesDumper(client)
		n, err := dumper.DumpAll(context.Background(), s.DumpDirAll)
		s.Require().NoError(err)
		s.Require().Greater(n, 0)
	} else {
		s.Require().NoError(err)
	}

	_, err = os.Stat(s.DumpDirPackage)
	if errors.Is(err, os.ErrNotExist) {
		client, err := kibana.NewClient()
		s.Require().NoError(err)

		dumper := NewAgentPoliciesDumper(client)
		n, err := dumper.DumpByPackage(context.Background(), s.DumpDirPackage, s.PackageName)
		s.Require().NoError(err)
		s.Require().Greater(n, 0)
	} else {
		s.Require().NoError(err)
	}

	_, err = os.Stat(s.DumpDirAgentPolicy)
	if errors.Is(err, os.ErrNotExist) {
		client, err := kibana.NewClient()
		s.Require().NoError(err)

		dumper := NewAgentPoliciesDumper(client)
		err = dumper.DumpByName(context.Background(), s.DumpDirAgentPolicy, s.AgentPolicy)
		s.Require().NoError(err)
	} else {
		s.Require().NoError(err)
	}
}

func (s *agentPoliciesDumpSuite) TestDumpAll() {
	client := kibana.NewTestClient(s.T(), s.RecordDir)

	outputDir := s.T().TempDir()
	dumper := NewAgentPoliciesDumper(client)
	n, err := dumper.DumpAll(context.Background(), outputDir)
	s.Require().NoError(err)

	filesExpected := countFiles(s.T(), s.DumpDirAll)
	s.Assert().Equal(filesExpected, n)

	filesFound := countFiles(s.T(), outputDir)
	s.Assert().Equal(filesExpected, filesFound)

	assertEqualDumps(s.T(), s.DumpDirAll, outputDir)
}

func (s *agentPoliciesDumpSuite) TestDumpByPackage() {
	client := kibana.NewTestClient(s.T(), s.RecordDir)

	outputDir := s.T().TempDir()
	dumper := NewAgentPoliciesDumper(client)
	n, err := dumper.DumpByPackage(context.Background(), outputDir, s.PackageName)
	s.Require().NoError(err)

	filesExpected := countFiles(s.T(), s.DumpDirPackage)
	s.Assert().Equal(filesExpected, n)

	filesFound := countFiles(s.T(), outputDir)
	s.Assert().Equal(filesExpected, filesFound)

	assertEqualDumps(s.T(), s.DumpDirPackage, outputDir)
}

func (s *agentPoliciesDumpSuite) TestDumpByName() {
	client := kibana.NewTestClient(s.T(), s.RecordDir)

	outputDir := s.T().TempDir()
	dumper := NewAgentPoliciesDumper(client)
	err := dumper.DumpByName(context.Background(), outputDir, s.AgentPolicy)
	s.Require().NoError(err)

	filesExpected := countFiles(s.T(), s.DumpDirAgentPolicy)
	s.Assert().Equal(filesExpected, 1)

	filesFound := countFiles(s.T(), outputDir)
	s.Assert().Equal(filesExpected, filesFound)

	assertEqualDumps(s.T(), s.DumpDirAgentPolicy, outputDir)
}
