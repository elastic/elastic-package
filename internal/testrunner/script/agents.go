// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package script

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"time"

	"github.com/rogpeppe/go-internal/testscript"

	"github.com/elastic/elastic-package/internal/agentdeployer"
	"github.com/elastic/elastic-package/internal/common"
	"github.com/elastic/elastic-package/internal/kibana"
	"github.com/elastic/elastic-package/internal/testrunner/runners/system"
)

func installAgent(ts *testscript.TestScript, neg bool, args []string) {
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

	flg := flag.NewFlagSet("install", flag.ContinueOnError)
	profName := flg.String("profile", "default", "profile name")
	timeout := flg.Duration("timeout", 0, "timeout (zero or lower indicates no timeout)")
	ts.Check(flg.Parse(args))
	if flg.NArg() != 0 && flg.NArg() != 1 {
		ts.Fatalf("usage: install_agent [-profile <profile>] [-timeout <duration>] [<network_name_label>]")
	}

	var networkNameLabel string
	if flg.NArg() == 1 {
		networkNameLabel = flg.Arg(0)
	}

	stk, ok := stacks[*profName]
	if !ok {
		ts.Fatalf("no active client for %s", *profName)
	}

	ctx := context.Background()
	if *timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, *timeout)
		defer cancel()
	}

	var installed installedAgent
	defer func() {
		r := recover()
		switch r := r.(type) {
		case nil:
			return
		case error:
			if errors.Is(r, failedRun) {
				ts.Check(decoratedWith("deleting failed policies", deletePolicies(ctx, stk.kibana, &installed)))
			}
		}
		panic(r)
	}()

	installed.started = time.Now()
	var err error
	installed.enrolledPolicy, err = stk.kibana.CreatePolicy(ctx, kibana.Policy{
		Name:        fmt.Sprintf("ep-test-system-enroll-%s-%s-%s-%s-%s", pkg, ds, "", ts.Name(), installed.started.Format("20060102T15:04:05Z")),
		Description: fmt.Sprintf("test policy created by elastic-package to enroll agent for data stream %s/%s", pkg, ds),
		Namespace:   common.CreateTestRunID(),
	})
	ts.Check(decoratedWith("creating kibana enrolled policy", err))
	installed.testingPolicy, err = stk.kibana.CreatePolicy(ctx, kibana.Policy{
		Name:        fmt.Sprintf("ep-test-system-%s-%s-%s-%s-%s", pkg, ds, "", ts.Name(), installed.started.Format("20060102T15:04:05Z")),
		Description: fmt.Sprintf("test policy created by elastic-package to enroll agent for data stream %s/%s", pkg, ds),
		Namespace:   common.CreateTestRunID(),
	})
	ts.Check(decoratedWith("creating kibana testing policy", err))

	dep, err := agentdeployer.NewCustomAgentDeployer(agentdeployer.DockerComposeAgentDeployerOptions{
		Profile:      stk.profile,
		StackVersion: stk.version,
		PackageName:  pkg,
		DataStream:   ds,
		PolicyName:   installed.enrolledPolicy.Name,
	})
	ts.Check(decoratedWith("making agent deployer", err))

	info := agentdeployer.AgentInfo{Name: pkg}
	info.Policy.Name = installed.enrolledPolicy.Name
	info.Policy.ID = installed.enrolledPolicy.ID
	info.Agent.AgentSettings.Runtime = "docker"
	info.Logs.Folder.Agent = system.ServiceLogsAgentDir
	info.Test.RunID = common.CreateTestRunID()
	info.Logs.Folder.Local, err = agentdeployer.CreateServiceLogsDir(stk.profile, pkgRoot, ds, info.Test.RunID)
	ts.Check(decoratedWith("creating service logs directory", err))

	// This will break for internal stacks if
	// ELASTIC_PACKAGE_CA_CERT is set. ¯\_(ツ)_/¯
	installed.deployed, err = dep.SetUp(ctx, info)
	ts.Check(decoratedWith("setting up agent", err))
	if networkNameLabel != "" {
		ts.Setenv(networkNameLabel, installed.deployed.Info().NetworkName)
	}
	polID := installed.deployed.Info().Policy.ID
	ts.Check(decoratedWith("getting kibana agent", doKibanaAgent(ctx, stk.kibana, func(a kibana.Agent) (bool, error) {
		if a.PolicyID != polID {
			return false, nil
		}
		installed.enrolled = a
		return true, nil
	})))
	ts.Check(decoratedWith("setting log level to debug", stk.kibana.SetAgentLogLevel(ctx, installed.enrolled.ID, "debug")))

	agents[*profName] = &installed
	fmt.Fprintf(ts.Stdout(), "installed agent policies for %s/%s\n", pkg, ds)
}

func doKibanaAgent(ctx context.Context, cli *kibana.Client, fn func(a kibana.Agent) (done bool, _ error)) error {
	for {
		enrolled, err := cli.QueryAgents(ctx, "")
		if err != nil {
			return decoratedWith("getting enrolled agents", err)
		}
		for _, a := range enrolled {
			if a.PolicyRevision == 0 || a.Status != "online" {
				continue
			}
			if done, err := fn(a); done || err != nil {
				return err
			}
		}
		time.Sleep(5 * time.Second)
	}
}

func uninstallAgent(ts *testscript.TestScript, neg bool, args []string) {
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
	agents, ok := ts.Value(installedAgentsTag{}).(map[string]*installedAgent)
	if !ok {
		ts.Fatalf("no installed installed agent registry")
	}

	flg := flag.NewFlagSet("uninstall", flag.ContinueOnError)
	profName := flg.String("profile", "default", "profile name")
	timeout := flg.Duration("timeout", 0, "timeout (zero or lower indicates no timeout)")
	ts.Check(flg.Parse(args))
	if flg.NArg() != 0 {
		ts.Fatalf("usage: uninstall_agent [-profile <profile>] [-timeout <duration>]")
	}

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

	delete(agents, *profName)

	ts.Check(decoratedWith("removing agent", stk.kibana.RemoveAgent(ctx, installed.enrolled)))
	ts.Check(decoratedWith("tearing down agent", installed.deployed.TearDown(ctx)))
	ts.Check(decoratedWith("deleting policies", deletePolicies(ctx, stk.kibana, installed)))

	fmt.Fprintf(ts.Stdout(), "deleted agent policies for %s/%s (testing:%s enrolled:%s)\n", pkg, ds, installed.testingPolicy.ID, installed.enrolledPolicy.ID)
}

type installedAgentsTag struct{}

type installedAgent struct {
	// agent details
	deployed agentdeployer.DeployedAgent
	enrolled kibana.Agent // ᕙ(⇀‸↼‶)ᕗ

	// policy details
	enrolledPolicy, testingPolicy *kibana.Policy

	started time.Time
}

func deletePolicies(ctx context.Context, cli *kibana.Client, a *installedAgent) error {
	var errs []error
	if a.testingPolicy != nil {
		errs = append(errs, cli.DeletePolicy(ctx, a.testingPolicy.ID))
	}
	if a.enrolledPolicy != nil {
		errs = append(errs, cli.DeletePolicy(ctx, a.enrolledPolicy.ID))
	}
	return errors.Join(errs...)
}
