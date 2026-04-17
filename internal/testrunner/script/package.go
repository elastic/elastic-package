// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package script

import (
	"archive/zip"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/rogpeppe/go-internal/testscript"

	"github.com/elastic/elastic-package/internal/fields"
	"github.com/elastic/elastic-package/internal/files"
	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/elastic-package/internal/registry"
	"github.com/elastic/elastic-package/internal/requiredinputs"
	"github.com/elastic/elastic-package/internal/resources"
	"github.com/elastic/elastic-package/internal/stack"
	"github.com/elastic/elastic-package/internal/testrunner"
)

func addPackage(ts *testscript.TestScript, neg bool, args []string) {
	clearStdStreams(ts)

	pkgRoot := ts.Getenv("PACKAGE_ROOT")
	if pkgRoot == "" {
		ts.Fatalf("PACKAGE_ROOT is not set")
	}
	root, err := files.FindRepositoryRootFrom(pkgRoot)
	ts.Check(err)
	pkg := ts.Getenv("PACKAGE_NAME")
	if pkg == "" {
		ts.Fatalf("PACKAGE_NAME is not set")
	}
	ecsBaseSchemaURL := ts.Getenv("ECS_BASE_SCHEMA_URL")
	if ecsBaseSchemaURL == "" {
		ts.Fatalf("ECS_BASE_SCHEMA_URL is not set")
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

	globalTestConfig, err := testrunner.ReadGlobalTestConfig(pkgRoot)
	ts.Check(err)

	registryBaseURL := ts.Getenv("PACKAGE_REGISTRY_BASE_URL")
	eprClient, err := registry.NewClient(registryBaseURL, stack.RegistryClientOptions(registryBaseURL, stk.profile)...)
	ts.Check(decoratedWith("creating package registry client", err))

	mergedOverrides, err := globalTestConfig.MergedRequiresSourceOverrides(pkgRoot)
	ts.Check(err)

	resolver := requiredinputs.NewRequiredInputsResolver(
		eprClient,
		requiredinputs.WithSourceOverrides(mergedOverrides),
	)

	m := resources.NewManager()
	m.RegisterProvider(resources.DefaultKibanaProviderName, &resources.KibanaProvider{Client: stk.kibana})
	_, err = m.ApplyCtx(ctx, resources.Resources{&resources.FleetPackage{
		PackageRoot:            pkgRoot,
		Absent:                 false,
		Force:                  true,
		RepositoryRoot:         root,
		SchemaURLs:             fields.NewSchemaURLs(fields.WithECSBaseURL(ecsBaseSchemaURL)),
		RequiredInputsResolver: resolver,
	}})
	ts.Check(decoratedWith("installing package resources", err))

	fmt.Fprintf(ts.Stdout(), "added package resources for %s\n", pkg)
}

func removePackage(ts *testscript.TestScript, neg bool, args []string) {
	clearStdStreams(ts)

	pkgRoot := ts.Getenv("PACKAGE_ROOT")
	if pkgRoot == "" {
		ts.Fatalf("PACKAGE_ROOT is not set")
	}
	root, err := files.FindRepositoryRootFrom(pkgRoot)
	ts.Check(err)
	pkg := ts.Getenv("PACKAGE_NAME")
	if pkg == "" {
		ts.Fatalf("PACKAGE_NAME is not set")
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
		PackageRoot:    pkgRoot,
		Absent:         true, // uninstall only — no bundling takes place, so no resolver is needed
		Force:          true,
		RepositoryRoot: root, // Apparently not required, but adding for safety.
	}})
	ts.Check(decoratedWith("removing package resources", err))

	fmt.Fprintf(ts.Stdout(), "removed package resources for %s\n", pkg)
}

func installPackageFromRegistry(ts *testscript.TestScript, neg bool, args []string) {
	clearStdStreams(ts)

	stacks, ok := ts.Value(runningStackTag{}).(map[string]*runningStack)
	if !ok {
		ts.Fatalf("no active stacks registry")
	}
	regPkgs, ok := ts.Value(registryPackagesTag{}).(map[string][]registryPackage)
	if !ok {
		ts.Fatalf("no registry packages registry")
	}
	regRoots, ok := ts.Value(registryPackageRootsTag{}).(map[string]string)
	if !ok {
		ts.Fatalf("no registry package roots registry")
	}

	flg := flag.NewFlagSet("install_registry", flag.ContinueOnError)
	profName := flg.String("profile", "default", "profile name")
	timeout := flg.Duration("timeout", 0, "timeout (zero or lower indicates no timeout)")
	ts.Check(flg.Parse(args))
	if flg.NArg() != 2 {
		ts.Fatalf("usage: install_package_from_registry [-profile <profile>] [-timeout <duration>] <name> <version>")
	}

	name := flg.Arg(0)
	version := flg.Arg(1)

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

	registryBaseURL := ts.Getenv("PACKAGE_REGISTRY_BASE_URL")
	if registryBaseURL == "" {
		ts.Fatalf("PACKAGE_REGISTRY_BASE_URL is not set")
	}

	_, err := stk.kibana.InstallPackage(ctx, name, version)
	ts.Check(decoratedWith("installing package from registry", err))

	regPkgs[*profName] = append(regPkgs[*profName], registryPackage{name: name, version: version})

	workDir := ts.MkAbs(".")
	client, err := registry.NewClient(registryBaseURL, stack.RegistryClientOptions(registryBaseURL, stk.profile)...)
	ts.Check(decoratedWith("creating package registry client", err))
	zipPath, err := client.DownloadPackage(name, version, workDir)
	ts.Check(decoratedWith("downloading package from registry", err))

	ts.Check(decoratedWith("extracting package zip", extractZip(zipPath, workDir)))

	key := fmt.Sprintf("%s-%s", name, version)
	eprRoot := filepath.Join(workDir, key)
	regRoots[key] = eprRoot

	fmt.Fprintf(ts.Stdout(), "installed %s %s from registry\n", name, version)
}

type registryPackagesTag struct{}
type registryPackageRootsTag struct{}

type registryPackage struct {
	name    string
	version string
}

// extractZip extracts a zip file to destDir. It validates that
// the zip contains exactly one top-level directory.
func extractZip(zipPath, destDir string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("opening zip %s: %w", zipPath, err)
	}
	defer r.Close()

	dirs, err := fs.ReadDir(r, ".")
	if err != nil {
		return fmt.Errorf("reading zip root: %w", err)
	}
	if len(dirs) != 1 || !dirs[0].IsDir() {
		return fmt.Errorf("expected exactly one top-level directory in zip, got %d entries", len(dirs))
	}

	for _, f := range r.File {
		name := strings.TrimSuffix(f.Name, "/")
		if !fs.ValidPath(name) {
			return fmt.Errorf("invalid path in zip: %s", f.Name)
		}
		if strings.Contains(name, "..") {
			return fmt.Errorf("path traversal in zip: %s", f.Name)
		}

		target := filepath.Join(destDir, name)
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return fmt.Errorf("creating directory %s: %w", target, err)
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return fmt.Errorf("creating parent directory for %s: %w", target, err)
		}

		src, err := f.Open()
		if err != nil {
			return fmt.Errorf("opening zip entry %s: %w", f.Name, err)
		}
		dst, err := os.Create(target)
		if err != nil {
			src.Close()
			return fmt.Errorf("creating file %s: %w", target, err)
		}
		_, err = io.Copy(dst, src)
		src.Close()
		if closeErr := dst.Close(); err == nil {
			err = closeErr
		}
		if err != nil {
			return fmt.Errorf("writing file %s: %w", target, err)
		}
	}
	return nil
}

func upgradePackageLatest(ts *testscript.TestScript, neg bool, args []string) {
	clearStdStreams(ts)

	pkg := ts.Getenv("PACKAGE_NAME")
	if pkg == "" {
		ts.Fatalf("PACKAGE_NAME is not set")
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

	pkg := ts.Getenv("PACKAGE_NAME")
	if pkg == "" {
		ts.Fatalf("PACKAGE_NAME is not set")
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

	pkg := ts.Getenv("PACKAGE_NAME")
	if pkg == "" {
		ts.Fatalf("PACKAGE_NAME is not set")
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
