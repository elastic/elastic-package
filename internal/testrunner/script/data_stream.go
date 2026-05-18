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

	"github.com/elastic/elastic-package/internal/builder"
	"github.com/elastic/elastic-package/internal/common"
	"github.com/elastic/elastic-package/internal/kibana"
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
	version := flg.String("version", "", "package version (reads manifests from EPR package extracted by install_package_from_registry)")
	timeout := flg.Duration("timeout", 0, "timeout (zero or lower indicates no timeout)")
	ts.Check(flg.Parse(args))
	if flg.NArg() != 2 {
		ts.Fatalf("usage: add_package_policy [-profile <profile>] [-timeout <duration>] [-policy <policy_name>] [-version <version>] <config.yaml> <name_var_label>")
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

	builtPkgRoot, pkgMan := resolveBuiltPackageRoot(ts, pkg, pkgRoot, *version)

	allDatastreams, err := packages.ReadAllDataStreamManifests(builtPkgRoot)
	ts.Check(decoratedWith("reading all data stream manifests", err))
	var dsMan *packages.DataStreamManifest
	for i := range allDatastreams {
		if allDatastreams[i].Name == ds {
			dsMan = &allDatastreams[i]
			break
		}
	}
	if dsMan == nil {
		ts.Fatalf("data stream %q not found in package %s", ds, pkgMan.Name)
	}

	if *polName == "" {
		*polName, err = packages.FindPolicyTemplateForInput(pkgMan, dsMan, config.Input)
		ts.Check(decoratedWith("finding policy template name", err))
	}
	templ, err := packages.SelectPolicyTemplateByName(pkgMan.PolicyTemplates, *polName)
	ts.Check(decoratedWith("finding policy template", err))

	policy, dsType, dsDataset, err := kibana.CreatePackagePolicy(installed.testingPolicy, *polName, dsMan.Name, config.Input, config.Vars, config.DataStream.Vars, installed.testingPolicy.Namespace, *pkgMan, dsMan, allDatastreams)
	ts.Check(decoratedWith("creating package policy", err))
	_, err = stk.kibana.CreatePackagePolicy(ctx, policy, kibana.PolicyAPIFormatAuto)
	ts.Check(decoratedWith("adding package policy", err))

	pol, err := stk.kibana.GetPolicy(ctx, installed.testingPolicy.ID)
	ts.Check(decoratedWith("reading policy", err))
	ts.Check(decoratedWith("assigning policy", stk.kibana.AssignPolicyToAgent(ctx, installed.enrolled, *pol)))

	dsName := system.BuildDataStreamName(dsType, dsDataset, installed.testingPolicy.Namespace, templ)
	ts.Setenv(dsNameLabel, dsName)
	dataStreams[dsName] = struct{}{}

	fmt.Fprintf(ts.Stdout(), "added %s data stream policy templates for %s/%s\n", dsName, pkg, ds)
}

// resolveBuiltPackageRoot returns a package root that is guaranteed to be a built
// package, together with its parsed manifest. There are two sources of a built
// package, selected by version:
//   - version == "": the locally built copy under build/packages/<name>/<version>/,
//     located via builder.ReadBuiltPackageManifest from pkgSourceRoot.
//   - version != "": the registry-extracted package populated by
//     install_package_from_registry (the package registry only serves built
//     packages, so the extract is already resolved).
//
// All subsequent manifest reads must come from a built package so that composable
// "package:" references are resolved into concrete input types.
func resolveBuiltPackageRoot(ts *testscript.TestScript, pkg, pkgSourceRoot, version string) (string, *packages.PackageManifest) {
	if version == "" {
		root, manifest, err := builder.ReadBuiltPackageManifest(pkgSourceRoot)
		ts.Check(decoratedWith("reading built package manifest", err))
		return root, manifest
	}
	regRoots, ok := ts.Value(registryPackageRootsTag{}).(map[string]string)
	if !ok {
		ts.Fatalf("no registry package roots registry")
	}
	key := fmt.Sprintf("%s-%s", pkg, version)
	root, ok := regRoots[key]
	if !ok {
		ts.Fatalf("no extracted EPR package for %s (call install_package_from_registry first)", key)
	}
	manifest, err := packages.ReadPackageManifestFromPackageRoot(root)
	ts.Check(decoratedWith("reading package manifest from registry extract", err))
	return root, manifest
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
	if _, err := io.Copy(&body, resp.Body); err != nil {
		ts.Fatalf("reading response body: %v", err)
	}
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
