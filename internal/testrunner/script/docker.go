// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package script

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/rogpeppe/go-internal/testscript"

	"github.com/elastic/elastic-package/internal/common"
	"github.com/elastic/elastic-package/internal/compose"
	"github.com/elastic/elastic-package/internal/configuration/locations"
	"github.com/elastic/elastic-package/internal/servicedeployer"
	"github.com/elastic/elastic-package/internal/testrunner/runners/system"
)

// dockerUp brings up a service using docker-compose.
func dockerUp(ts *testscript.TestScript, neg bool, args []string) {
	clearStdStreams(ts)

	if neg {
		ts.Fatalf("unsupported: ! docker_up")
	}

	stacks, ok := ts.Value(runningStackTag{}).(map[string]*runningStack)
	if !ok {
		ts.Fatalf("no active stacks registry")
	}
	srvs, ok := ts.Value(deployedServiceTag{}).(map[string]servicedeployer.DeployedService)
	if !ok {
		ts.Fatalf("no deployed services registry")
	}

	flg := flag.NewFlagSet("up", flag.ContinueOnError)
	profName := flg.String("profile", "default", "profile name")
	netName := flg.String("network", "", "network name")
	timeout := flg.Duration("timeout", 0, "timeout (zero or lower indicates no timeout)")
	ts.Check(flg.Parse(args))
	if flg.NArg() != 1 {
		ts.Fatalf("usage: docker_up [-profile <profile>] [-timeout <duration>] <dir>")
	}
	name := flg.Arg(0)
	dir := ts.MkAbs(name)
	compose := filepath.Join(dir, "docker-compose.yml")
	_, err := os.Stat(compose)
	ts.Check(err)

	ctx := context.Background()
	if *timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, *timeout)
		defer cancel()
	}

	stk, ok := stacks[*profName]
	if !ok {
		ts.Fatalf("no active client for %s", *profName)
	}

	dep, err := servicedeployer.NewDockerComposeServiceDeployer(servicedeployer.DockerComposeServiceDeployerOptions{
		Profile:                stk.profile,
		YmlPaths:               []string{compose},
		DeployIndependentAgent: *netName != "",
	})
	ts.Check(decoratedWith("making service deployer", err))

	loc, err := locations.NewLocationManager()
	ts.Check(err)

	info := servicedeployer.ServiceInfo{
		Name:             name,
		AgentNetworkName: *netName,
	}
	info.Logs.Folder.Agent = system.ServiceLogsAgentDir
	info.Logs.Folder.Local = ts.MkAbs(loc.ServiceLogDir())
	info.Test.RunID = common.CreateTestRunID()

	up, err := dep.SetUp(ctx, info)
	ts.Check(decoratedWith("setting up service", err))
	srvs[name] = up
	fmt.Fprintf(ts.Stdout(), "deployed %s:%s-1", info.ProjectName(), name)
}

// dockerSignal sends a signal to the named service.
func dockerSignal(ts *testscript.TestScript, neg bool, args []string) {
	clearStdStreams(ts)

	if neg {
		ts.Fatalf("unsupported: ! docker_signal")
	}

	srvs, ok := ts.Value(deployedServiceTag{}).(map[string]servicedeployer.DeployedService)
	if !ok {
		ts.Fatalf("no deployed services registry")
	}

	flg := flag.NewFlagSet("signal", flag.ContinueOnError)
	timeout := flg.Duration("timeout", 0, "timeout (zero or lower indicates no timeout)")
	ts.Check(flg.Parse(args))
	if flg.NArg() != 2 {
		ts.Fatalf("usage: docker_signal [-timeout <duration>] <name> <signal>")
	}
	name := flg.Arg(0)
	signal := flg.Arg(1)

	up, ok := srvs[name]
	if !ok {
		ts.Fatalf("service %s is not deployed", name)
	}

	ctx := context.Background()
	if *timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, *timeout)
		defer cancel()
	}
	ts.Check(decoratedWith("sending signal", up.Signal(ctx, signal)))
}

// dockerWaitExit waits for the service to exit
func dockerWaitExit(ts *testscript.TestScript, neg bool, args []string) {
	clearStdStreams(ts)

	if neg {
		ts.Fatalf("unsupported: ! docker_wait_exit")
	}

	srvs, ok := ts.Value(deployedServiceTag{}).(map[string]servicedeployer.DeployedService)
	if !ok {
		ts.Fatalf("no deployed services registry")
	}

	flg := flag.NewFlagSet("exit", flag.ContinueOnError)
	timeout := flg.Duration("timeout", 0, "timeout (zero or lower indicates no timeout)")
	ts.Check(flg.Parse(args))
	if flg.NArg() != 1 {
		ts.Fatalf("usage: docker_wait_exit [-timeout <duration>] <name>")
	}
	name := flg.Arg(0)

	up, ok := srvs[name]
	if !ok {
		ts.Fatalf("service %s is not deployed", name)
	}

	ctx := context.Background()
	if *timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, *timeout)
		defer cancel()
	}
	for {
		done, code, err := up.ExitCode(ctx, name)
		ts.Check(decoratedWith("checking exit", err))
		if done {
			fmt.Fprintf(ts.Stdout(), "%s exited with %d", name, code)
			return
		}
	}
}

// dockerDown takes down a deployed service and emits the service's logs to stdout.
func dockerDown(ts *testscript.TestScript, neg bool, args []string) {
	clearStdStreams(ts)

	if neg {
		ts.Fatalf("unsupported: ! docker_down")
	}

	srvs, ok := ts.Value(deployedServiceTag{}).(map[string]servicedeployer.DeployedService)
	if !ok {
		ts.Fatalf("no deployed services registry")
	}

	flg := flag.NewFlagSet("down", flag.ContinueOnError)
	timeout := flg.Duration("timeout", 0, "timeout (zero or lower indicates no timeout)")
	ts.Check(flg.Parse(args))
	if flg.NArg() != 1 {
		ts.Fatalf("usage: docker_down [-timeout <duration>] <name>")
	}
	name := flg.Arg(0)

	up, ok := srvs[name]
	if !ok {
		ts.Fatalf("service %s is not deployed", name)
	}

	ctx := context.Background()
	if *timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, *timeout)
		defer cancel()
	}
	ts.Check(decoratedWith("writing logs", writeLogsTo(ctx, ts.Stdout(), up)))
	ts.Check(decoratedWith("stopping service", up.TearDown(ctx)))
	delete(srvs, name)
}

type deployedServiceTag struct{}

func writeLogsTo(ctx context.Context, w io.Writer, s servicedeployer.DeployedService) error {
	p, err := projectFor(s)
	if err != nil {
		return err
	}
	env, err := envOf(s)
	if err != nil {
		return err
	}
	b, err := p.Logs(ctx, compose.CommandOptions{Env: env})
	if err != nil {
		return err
	}
	_, err = w.Write(b)
	return err
}

func projectFor(s servicedeployer.DeployedService) (*compose.Project, error) {
	type projector interface {
		Project() (*compose.Project, error)
	}
	p, ok := s.(projector)
	if !ok {
		return nil, fmt.Errorf("cannot get project from %T", s)
	}
	return p.Project()
}

func envOf(s servicedeployer.DeployedService) ([]string, error) {
	type enver interface {
		Env() []string
	}
	e, ok := s.(enver)
	if !ok {
		return nil, fmt.Errorf("cannot get environment from %T", s)
	}
	return e.Env(), nil
}
