// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package script

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/rogpeppe/go-internal/testscript"

	"github.com/elastic/go-ucfg"
	"github.com/elastic/go-ucfg/yaml"

	"github.com/elastic/elastic-package/internal/common"
	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/elastic-package/internal/testrunner/runners/system"
)

func addPackagePolicy(ts *testscript.TestScript, neg bool, args []string) {
	clearStdStreams(ts)

	pkgRoot := ts.Getenv("PACKAGE_ROOT")
	if pkgRoot == "" {
		ts.Fatalf("PACKAGE_ROOT is not set")
	}
	pkg := ts.Getenv("PACKAGE_NAME")
	if pkg == "" {
		ts.Fatalf("PACKAGE_NAME is not set")
	}
	ds := ts.Getenv("DATA_STREAM")
	if ds == "" {
		ts.Fatalf("DATA_STREAM is not set")
	}

	stacks, ok := ts.Value(runningStackTag{}).(map[string]*runningStack)
	if !ok {
		ts.Fatalf("no active stacks registry")
	}
	agents, ok := ts.Value(installedAgentsTag{}).(map[string]*installedAgent)
	if !ok {
		ts.Fatalf("no installed installed agent registry")
	}
	dataStreams, ok := ts.Value(installedDataStreamsTag{}).(map[string]struct{})
	if !ok {
		ts.Fatalf("no installed installed data streams registry")
	}

	flg := flag.NewFlagSet("add", flag.ContinueOnError)
	profName := flg.String("profile", "default", "profile name")
	polName := flg.String("policy", "", "policy name")
	timeout := flg.Duration("timeout", 0, "timeout (zero or lower indicates no timeout)")
	ts.Check(flg.Parse(args))
	if flg.NArg() != 2 {
		ts.Fatalf("usage: add_package_policy [-profile <profile>] [-timeout <duration>] [-policy <policy_name>] <config.yaml> <name_var_label>")
	}

	cfgPath := ts.MkAbs(flg.Arg(0))
	dsNameLabel := flg.Arg(1)

	cfgData, err := os.ReadFile(cfgPath)
	ts.Check(decoratedWith("reading data stream configuration", err))
	cfg, err := yaml.NewConfig(cfgData, ucfg.PathSep("."))
	ts.Check(decoratedWith("deserializing data stream configuration", err))
	var config struct {
		Input      string        `config:"input"`
		Vars       common.MapStr `config:"vars"`
		DataStream struct {
			Vars common.MapStr `config:"vars"`
		} `config:"data_stream"`
	}
	ts.Check(decoratedWith("unpacking configuration", cfg.Unpack(&config)))

	stk, ok := stacks[*profName]
	if !ok {
		ts.Fatalf("no active client for %s", *profName)
	}
	installed, ok := agents[*profName]
	if !ok {
		ts.Fatalf("agent policy in %s is not installed", *profName)
	}

	ctx := context.Background()
	if *timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, *timeout)
		defer cancel()
	}

	pkgMan, err := packages.ReadPackageManifestFromPackageRoot(pkgRoot)
	ts.Check(decoratedWith("reading package manifest", err))
	dsMan, err := packages.ReadDataStreamManifestFromPackageRoot(pkgRoot, ds)
	ts.Check(decoratedWith("reading data stream manifest", err))

	if *polName == "" {
		*polName, err = system.FindPolicyTemplateForInput(pkgMan, dsMan, config.Input)
		ts.Check(decoratedWith("finding policy template name", err))
	}
	templ, err := system.SelectPolicyTemplateByName(pkgMan.PolicyTemplates, *polName)
	ts.Check(decoratedWith("finding policy template", err))

	policy, dsType, dsDataset, err := system.CreatePackagePolicy(installed.testingPolicy, pkgMan, templ, dsMan, config.Input, config.Vars, config.DataStream.Vars, installed.testingPolicy.Namespace)
	ts.Check(decoratedWith("creating package policy", err))
	_, err = stk.kibana.CreatePackagePolicy(ctx, policy)
	ts.Check(decoratedWith("adding package policy", err))

	pol, err := stk.kibana.GetPolicy(ctx, installed.testingPolicy.ID)
	ts.Check(decoratedWith("reading policy", err))
	ts.Check(decoratedWith("assigning policy", stk.kibana.AssignPolicyToAgent(ctx, installed.enrolled, *pol)))

	dsName := system.BuildDataStreamName(dsType, dsDataset, installed.testingPolicy.Namespace, templ, pkgMan.Type, config.Vars)
	ts.Setenv(dsNameLabel, dsName)
	dataStreams[dsName] = struct{}{}

	fmt.Fprintf(ts.Stdout(), "added %s data stream policy templates for %s/%s\n", dsName, pkg, ds)
}

func removePackagePolicy(ts *testscript.TestScript, neg bool, args []string) {
	clearStdStreams(ts)

	pkg := ts.Getenv("PACKAGE_NAME")
	if pkg == "" {
		ts.Fatalf("PACKAGE_NAME is not set")
	}
	ds := ts.Getenv("DATA_STREAM")
	if ds == "" {
		ts.Fatalf("DATA_STREAM is not set")
	}

	stacks, ok := ts.Value(runningStackTag{}).(map[string]*runningStack)
	if !ok {
		ts.Fatalf("no active stacks registry")
	}
	dataStreams, ok := ts.Value(installedDataStreamsTag{}).(map[string]struct{})
	if !ok {
		ts.Fatalf("no installed installed data streams registry")
	}

	flg := flag.NewFlagSet("remove", flag.ContinueOnError)
	profName := flg.String("profile", "default", "profile name")
	timeout := flg.Duration("timeout", 0, "timeout (zero or lower indicates no timeout)")
	ts.Check(flg.Parse(args))
	if flg.NArg() != 1 {
		ts.Fatalf("usage: remove_package_policy [-profile <profile>] [-timeout <duration>] <data_stream_name>")
	}

	dsName := flg.Arg(0)

	stk, ok := stacks[*profName]
	if !ok {
		ts.Fatalf("no active client for %s", *profName)
	}
	_, ok = dataStreams[dsName]
	if !ok {
		ts.Fatalf("no data stream for %s", dsName)
	}

	ctx := context.Background()
	if *timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, *timeout)
		defer cancel()
	}

	resp, err := stk.es.Indices.DeleteDataStream([]string{dsName},
		stk.es.Indices.DeleteDataStream.WithContext(ctx),
	)
	ts.Check(decoratedWith("requesting data stream removal for "+dsName, err))
	defer resp.Body.Close()
	var body bytes.Buffer
	io.Copy(&body, resp.Body)
	if resp.StatusCode == http.StatusNotFound {
		// Data stream doesn't exist, there was nothing to do.
		fmt.Fprintf(ts.Stderr(), "%s data stream policy templates do not exist for %s/%s\n", dsName, pkg, ds)
		return
	}
	if resp.StatusCode >= 300 {
		ts.Fatalf("delete request failed for data stream %s: %s", dsName, body.Bytes())
	}

	delete(dataStreams, dsName)

	fmt.Fprintf(ts.Stdout(), "removed %s data stream policy templates for %s/%s\n", dsName, pkg, ds)
}

type installedDataStreamsTag struct{}
