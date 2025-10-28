// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package script

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/rogpeppe/go-internal/testscript"

	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/elastic-package/internal/resources"
)

func addPackage(ts *testscript.TestScript, neg bool, args []string) {
	clearStdStreams(ts)

	pkgRoot := ts.Getenv("PKG_ROOT")
	if pkgRoot == "" {
		ts.Fatalf("PKG_ROOT is not set")
	}
	root, err := os.OpenRoot(pkgRoot)
	ts.Check(err)
	pkg := ts.Getenv("PKG")
	if pkg == "" {
		ts.Fatalf("PKG is not set")
	}

	stacks, ok := ts.Value(runningStackTag{}).(map[string]*runningStack)
	if !ok {
		ts.Fatalf("no active stacks registry")
	}

	flg := flag.NewFlagSet("add", flag.ContinueOnError)
	profName := flg.String("profile", "default", "profile name")
	timeout := flg.Duration("timeout", 0, "timeout (zero or lower indicates no timeout)")
	ts.Check(flg.Parse(args))
	if flg.NArg() != 0 {
		ts.Fatalf("usage: add_package [-profile <profile>] [-timeout <duration>]")
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

	m := resources.NewManager()
	m.RegisterProvider(resources.DefaultKibanaProviderName, &resources.KibanaProvider{Client: stk.kibana})
	_, err = m.ApplyCtx(ctx, resources.Resources{&resources.FleetPackage{
		PackageRootPath: pkgRoot,
		Absent:          false,
		Force:           true,
		RepositoryRoot:  root,
	}})
	ts.Check(decoratedWith("installing package resources", err))

	fmt.Fprintf(ts.Stdout(), "added package resources for %s\n", pkg)
}

func removePackage(ts *testscript.TestScript, neg bool, args []string) {
	clearStdStreams(ts)

	pkgRoot := ts.Getenv("PKG_ROOT")
	if pkgRoot == "" {
		ts.Fatalf("PKG_ROOT is not set")
	}
	root, err := os.OpenRoot(pkgRoot)
	ts.Check(err)
	pkg := ts.Getenv("PKG")
	if pkg == "" {
		ts.Fatalf("PKG is not set")
	}

	stacks, ok := ts.Value(runningStackTag{}).(map[string]*runningStack)
	if !ok {
		ts.Fatalf("no active stacks registry")
	}

	flg := flag.NewFlagSet("remove", flag.ContinueOnError)
	profName := flg.String("profile", "default", "profile name")
	timeout := flg.Duration("timeout", 0, "timeout (zero or lower indicates no timeout)")
	ts.Check(flg.Parse(args))
	if flg.NArg() != 0 {
		ts.Fatalf("usage: remove_package [-profile <profile>] [-timeout <duration>]")
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

	m := resources.NewManager()
	m.RegisterProvider(resources.DefaultKibanaProviderName, &resources.KibanaProvider{Client: stk.kibana})
	_, err = m.ApplyCtx(ctx, resources.Resources{&resources.FleetPackage{
		PackageRootPath: pkgRoot,
		Absent:          true,
		Force:           true,
		RepositoryRoot:  root, // Apparently not required, but adding for safety.
	}})
	ts.Check(decoratedWith("removing package resources", err))

	fmt.Fprintf(ts.Stdout(), "removed package resources for %s\n", pkg)
}

func upgradePackageLatest(ts *testscript.TestScript, neg bool, args []string) {
	clearStdStreams(ts)

	pkg := ts.Getenv("PKG")
	if pkg == "" {
		ts.Fatalf("PKG is not set")
	}

	stacks, ok := ts.Value(runningStackTag{}).(map[string]*runningStack)
	if !ok {
		ts.Fatalf("no active stacks registry")
	}

	flg := flag.NewFlagSet("upgrade", flag.ContinueOnError)
	profName := flg.String("profile", "default", "profile name")
	timeout := flg.Duration("timeout", 0, "timeout (zero or lower indicates no timeout)")
	ts.Check(flg.Parse(args))
	if flg.NArg() != 0 && flg.NArg() != 1 {
		ts.Fatalf("usage: upgrade_package_latest [-profile <profile>] [-timeout <duration>] [<package_name>]")
	}

	if flg.NArg() == 1 {
		pkg = flg.Arg(0)
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

	msgs, err := stk.kibana.ListRawPackagePolicies(ctx)
	ts.Check(decoratedWith("upgrade package", err))
	var n int
	for _, m := range msgs {
		var pol struct {
			ID      string `json:"id"`
			Package struct {
				Name    string `json:"name"`
				Version string `json:"version"`
			} `json:"package"`
		}
		ts.Check(decoratedWith("getting package policy id", json.Unmarshal(m, &pol)))
		if pol.Package.Name == pkg {
			n++
			ts.Check(decoratedWith("upgrading package policy", stk.kibana.UpgradePackagePolicyToLatest(ctx, pol.ID)))
			fmt.Fprintf(ts.Stdout(), "upgraded package %s from version %s\n", pkg, pol.Package.Version)
		}
	}
	if n == 0 {
		ts.Fatalf("could not find policy for %s", pkg)
	}
}

func addPackageZip(ts *testscript.TestScript, neg bool, args []string) {
	clearStdStreams(ts)

	pkg := ts.Getenv("PKG")
	if pkg == "" {
		ts.Fatalf("PKG is not set")
	}

	stacks, ok := ts.Value(runningStackTag{}).(map[string]*runningStack)
	if !ok {
		ts.Fatalf("no active stacks registry")
	}

	flg := flag.NewFlagSet("add", flag.ContinueOnError)
	profName := flg.String("profile", "default", "profile name")
	timeout := flg.Duration("timeout", 0, "timeout (zero or lower indicates no timeout)")
	ts.Check(flg.Parse(args))
	if flg.NArg() != 1 {
		ts.Fatalf("usage: add_package_zip [-profile <profile>] [-timeout <duration>] <path_to_zip>")
	}

	path := ts.MkAbs(flg.Arg(0))

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

	m, err := packages.ReadPackageManifestFromZipPackage(path)
	ts.Check(decoratedWith("reading zip manifest", err))

	_, err = stk.kibana.InstallZipPackage(ctx, path)
	ts.Check(decoratedWith("installing package zip", err))

	fmt.Fprintf(ts.Stdout(), "added zipped package resources in %s for %s in test for %s\n", path, m.Name, pkg)
}

func removePackageZip(ts *testscript.TestScript, neg bool, args []string) {
	clearStdStreams(ts)

	pkg := ts.Getenv("PKG")
	if pkg == "" {
		ts.Fatalf("PKG is not set")
	}

	stacks, ok := ts.Value(runningStackTag{}).(map[string]*runningStack)
	if !ok {
		ts.Fatalf("no active stacks registry")
	}

	flg := flag.NewFlagSet("remove_zip", flag.ContinueOnError)
	profName := flg.String("profile", "default", "profile name")
	timeout := flg.Duration("timeout", 0, "timeout (zero or lower indicates no timeout)")
	ts.Check(flg.Parse(args))
	if flg.NArg() != 1 {
		ts.Fatalf("usage: remove_package_zip [-profile <profile>] [-timeout <duration>] <path_to_zip>")
	}

	path := ts.MkAbs(flg.Arg(0))

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

	m, err := packages.ReadPackageManifestFromZipPackage(path)
	ts.Check(decoratedWith("reading zip manifest", err))

	_, err = stk.kibana.RemovePackage(ctx, m.Name, m.Version)
	ts.Check(decoratedWith("removing package zip", err))

	fmt.Fprintf(ts.Stdout(), "removed zipped package resources in %s for %s in test for %s\n", path, m.Name, pkg)
}
