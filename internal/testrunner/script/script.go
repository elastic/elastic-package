// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package script

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync/atomic"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/rogpeppe/go-internal/testscript"

	"github.com/elastic/elastic-package/internal/configuration/locations"
	"github.com/elastic/elastic-package/internal/elasticsearch/ingest"
	"github.com/elastic/elastic-package/internal/install"
	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/elastic-package/internal/packages/changelog"
	"github.com/elastic/elastic-package/internal/profile"
	"github.com/elastic/elastic-package/internal/registry"
	"github.com/elastic/elastic-package/internal/resources"
	"github.com/elastic/elastic-package/internal/servicedeployer"
	"github.com/elastic/elastic-package/internal/stack"
	"github.com/elastic/elastic-package/internal/testrunner"
)

// Options is the script testing configuration type.
type Options struct {
	Package string // The package being tested.

	Dir     string   // Path to directory containing script tests.
	Streams []string // Data streams to test.

	ExternalStack   bool   // Stack is provided externally to the scripts.
	RunPattern      string // Regular expression to select tests to run.
	Verbose         bool   // Verbose script logging.
	UpdateScripts   bool   // testscript.Params.UpdateScripts
	ContinueOnError bool   // testscript.Params.ContinueOnError
	TestWork        bool   // testscript.Params.TestWork

	// Profile selects the package registry URL from profile config (with app
	// config as fallback). When nil, the current profile name from application
	// configuration is loaded.
	Profile *profile.Profile
}

func profileAndPackageRegistryBaseURL(opt Options, appConfig *install.ApplicationConfiguration) (*profile.Profile, string, error) {
	prof := opt.Profile
	if prof == nil {
		var err error
		prof, err = profile.LoadProfile(appConfig.CurrentProfile())
		if err != nil {
			return nil, "", fmt.Errorf("loading profile %q: %w", appConfig.CurrentProfile(), err)
		}
	}
	return prof, stack.PackageRegistryBaseURL(prof, appConfig), nil
}

func revisionsFromRegistry(eprBaseURL string, prof *profile.Profile, pkgName string) ([]packages.PackageManifest, error) {
	c, err := registry.NewClient(eprBaseURL, stack.RegistryClientOptions(eprBaseURL, prof)...)
	if err != nil {
		return nil, fmt.Errorf("creating package registry client: %w", err)
	}
	return c.Revisions(pkgName, registry.SearchOptions{})
}

func scriptTestWorkdirRoot(workRoot string, opt Options) (workdirRoot string, err error) {
	if opt.TestWork {
		return os.MkdirTemp(workRoot, "*")
	}
	if err := os.Setenv("GOTMPDIR", workRoot); err != nil {
		return "", fmt.Errorf("could not set temp dir var: %w", err)
	}
	return "", nil
}

func Run(dst *[]testrunner.TestResult, w io.Writer, opt Options) error {
	if opt.Dir != "" && len(opt.Streams) != 0 {
		// We should never reach here.
		return errors.New("script directory path set with streams list")
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("could not find home: %w", err)
	}
	appConfig, err := install.Configuration()
	if err != nil {
		return fmt.Errorf("could read configuration: %w", err)
	}
	prof, eprBaseURL, err := profileAndPackageRegistryBaseURL(opt, appConfig)
	if err != nil {
		return err
	}
	loc, err := locations.NewLocationManager()
	if err != nil {
		return err
	}
	workRoot := filepath.Join(loc.TempDir(), "script_tests")
	err = os.MkdirAll(workRoot, 0o700)
	if err != nil {
		return fmt.Errorf("could not make work space root: %w", err)
	}
	// Only pass a non-zero work root when --work is set; otherwise set $GOTMPDIR
	// so testscript uses a directory we can operate in (see scriptTestWorkdirRoot).
	workdirRoot, err := scriptTestWorkdirRoot(workRoot, opt)
	if err != nil {
		if opt.TestWork {
			return fmt.Errorf("could not make work space: %w", err)
		}
		return err
	}

	pkgInfo, err := resolvePackageInfo(opt)
	if err != nil {
		return err
	}
	if len(pkgInfo.dirs) == 0 {
		return nil
	}

	t, err := newT(opt, dst, w)
	if err != nil {
		return err
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	manifest, latestEPRVersion, isLatestVersion, err := resolveLatestVersion(pkgInfo, eprBaseURL, prof)
	if err != nil {
		return err
	}
	if manifest != nil {
		t.Log("PKG ", filepath.Base(pkgInfo.root))
	}

	scriptCmds := scriptTestCommands()

	// Per-stream keys (DATA_STREAM, DATA_STREAM_ROOT) are added inside the loop via maps.Clone.
	baseEnv := map[string]string{
		"PROFILE":                   appConfig.CurrentProfile(),
		"CONFIG_ROOT":               loc.RootDir(),
		"CONFIG_PROFILES":           loc.ProfileDir(),
		"HOME":                      home,
		"ECS_BASE_SCHEMA_URL":       appConfig.SchemaURLs().ECSBase(),
		"PACKAGE_REGISTRY_BASE_URL": eprBaseURL,
	}
	if pkgInfo.root != "" {
		baseEnv["PACKAGE_NAME"] = manifest.Name
		baseEnv["PACKAGE_BASE"] = filepath.Base(pkgInfo.root)
		baseEnv["PACKAGE_ROOT"] = pkgInfo.root
	}
	if latestEPRVersion != "" {
		baseEnv["LATEST_EPR_VERSION"] = latestEPRVersion
	}
	if pkgInfo.currVersion != "" {
		baseEnv["CURRENT_VERSION"] = pkgInfo.currVersion
	}
	if pkgInfo.prevVersion != "" {
		baseEnv["PREVIOUS_VERSION"] = pkgInfo.prevVersion
	}

	var n int
	for _, d := range pkgInfo.dirs {
		ran, err := runScriptTestsForDir(ctx, t, d, pkgInfo, workdirRoot, baseEnv, scriptCmds, isLatestVersion, opt)
		if err != nil {
			return err
		}
		if ran {
			n++
		}
	}
	if n == 0 {
		t.Log("[no test files]")
	}
	return nil
}

// packageInfo is the complete result of script test resolution: which directories to run,
// the package root (empty when driven by an explicit --dir path), and changelog-derived
// metadata. Version and breaking-change fields are zero-valued when there is no package root.
type packageInfo struct {
	dirs           []string
	root           string
	currVersion    string
	prevVersion    string
	currSemver     *semver.Version // nil when no changelog entry exists
	breakingChange bool
}

// resolvePackageInfo resolves all information needed to run script tests: the target
// directories, the package root (when applicable), and changelog-derived version metadata.
// It handles both an explicit --dir path and data-stream discovery from a package root.
func resolvePackageInfo(opt Options) (packageInfo, error) {
	explicitDirs, err := scripts(opt.Dir)
	if err != nil {
		return packageInfo{}, err
	}
	if len(explicitDirs) != 0 {
		return packageInfo{dirs: explicitDirs}, nil
	}

	pkgRoot, err := packages.FindPackageRoot()
	if errors.Is(err, packages.ErrPackageRootNotFound) {
		return packageInfo{}, errors.New("package root not found")
	}
	if err != nil {
		return packageInfo{}, fmt.Errorf("locating package root failed: %w", err)
	}
	dirs, err := datastreams(slices.Clone(opt.Streams), pkgRoot)
	if err != nil {
		return packageInfo{}, err
	}
	info := packageInfo{dirs: dirs, root: pkgRoot}
	if len(dirs) == 0 {
		return info, nil
	}
	revs, err := changelog.ReadChangelogFromPackageRoot(pkgRoot)
	if err != nil {
		return packageInfo{}, err
	}
	if len(revs) > 0 {
		info.currVersion = revs[0].Version
		for _, c := range revs[0].Changes {
			if c.Type == "breaking-change" {
				info.breakingChange = true
				break
			}
		}
		info.currSemver, err = semver.NewVersion(info.currVersion)
		if err != nil {
			return packageInfo{}, fmt.Errorf("failed to parse current version: %w", err)
		}
	}
	if len(revs) > 1 {
		info.prevVersion = revs[1].Version
		if !info.breakingChange {
			prevSemver, err := semver.NewVersion(info.prevVersion)
			if err != nil {
				return packageInfo{}, fmt.Errorf("failed to parse previous version: %w", err)
			}
			info.breakingChange = info.currSemver.Major() != prevSemver.Major()
		}
	}
	return info, nil
}

// buildScriptCondition returns the testscript condition evaluator for the given run context.
func buildScriptCondition(opt Options, scriptEnv map[string]string, breakingChange, isLatestVersion bool, prevVersion string) func(string) (bool, error) {
	return func(cond string) (bool, error) {
		switch {
		case cond == "external_stack":
			return opt.ExternalStack, nil
		case cond == "breaking_change":
			return breakingChange, nil
		case cond == "is_latest_version":
			return isLatestVersion, nil
		case cond == "has_previous_release":
			return prevVersion != "", nil
		case strings.HasPrefix(cond, "env:"):
			_, ok := scriptEnv[cond[len("env:"):]]
			return ok, nil
		default:
			return false, fmt.Errorf("unknown condition: %s", cond)
		}
	}
}

// scriptTestCommands returns the map of custom testscript commands.
// Built once and shared across all data stream test runs.
func scriptTestCommands() map[string]func(ts *testscript.TestScript, neg bool, args []string) {
	return map[string]func(ts *testscript.TestScript, neg bool, args []string){
		"sleep":                         sleep,
		"date":                          date,
		"GET":                           get,
		"POST":                          post,
		"stack_up":                      stackUp,
		"use_stack":                     useStack,
		"stack_down":                    stackDown,
		"docker_up":                     dockerUp,
		"docker_down":                   dockerDown,
		"docker_signal":                 dockerSignal,
		"docker_wait_exit":              dockerWaitExit,
		"install_pipelines":             installPipelines,
		"simulate":                      simulate,
		"uninstall_pipelines":           uninstallPipelines,
		"install_agent":                 installAgent,
		"add_package":                   addPackage,
		"remove_package":                removePackage,
		"upgrade_package_latest":        upgradePackageLatest,
		"add_package_zip":               addPackageZip,
		"install_package_from_registry": installPackageFromRegistry,
		"remove_package_zip":            removePackageZip,
		"add_package_policy":            addPackagePolicy,
		"remove_package_policy":         removePackagePolicy,
		"uninstall_agent":               uninstallAgent,
		"get_docs":                      getDocs,
		"dump_logs":                     dumpLogs,
		"match_file":                    match,
		"get_policy":                    getPolicyCommand,
		"compile_registry_state":        compileRegistryState,
	}
}

// runScriptTestsForDir runs the script tests for a single data stream directory d.
// It returns true when test files were found and executed.
func runScriptTestsForDir(ctx context.Context, t *T, d string, pkgInfo packageInfo, workdirRoot string, baseEnv map[string]string, scriptCmds map[string]func(*testscript.TestScript, bool, []string), isLatestVersion bool, opt Options) (bool, error) {
	t.dataStream = d
	scripts := d
	var dsRoot string
	if pkgInfo.root != "" {
		dsRoot = filepath.Join(pkgInfo.root, "data_stream", d)
		scripts = filepath.Join(dsRoot, filepath.FromSlash("_dev/test/scripts"))
	}
	if _, err := os.Stat(scripts); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("checking scripts directory %q: %w", scripts, err)
	}

	scriptEnv := maps.Clone(baseEnv)
	if dsRoot != "" {
		scriptEnv["DATA_STREAM"] = d
		scriptEnv["DATA_STREAM_ROOT"] = dsRoot
	}
	p := testscript.Params{
		Dir:             scripts,
		WorkdirRoot:     workdirRoot,
		UpdateScripts:   opt.UpdateScripts,
		ContinueOnError: opt.ContinueOnError,
		TestWork:        opt.TestWork,
		Cmds:            scriptCmds,
		Setup: func(e *testscript.Env) error {
			for k, v := range scriptEnv {
				e.Setenv(k, v)
			}
			e.Values[deployedServiceTag{}] = t.deployedService
			e.Values[runningStackTag{}] = t.runningStack
			e.Values[installedAgentsTag{}] = t.installedAgents
			e.Values[installedDataStreamsTag{}] = t.installedDataStreams
			e.Values[installedPipelinesTag{}] = t.installedPipelines
			e.Values[registryPackagesTag{}] = t.registryPackages
			e.Values[registryPackageRootsTag{}] = t.registryPackageRoots
			return nil
		},
		Condition: buildScriptCondition(opt, scriptEnv, pkgInfo.breakingChange, isLatestVersion, pkgInfo.prevVersion),
	}
	// This is not the ideal approach. What I would like would
	// be to pass this into the testscript, but that is a bunch
	// of wiring and likely should either be added later when
	// needed, or have the option of passing it in to the library
	// upstream.
	if ctx.Err() != nil {
		t.Fatal("interrupted")
	}
	t.Log("DATA_STREAM ", d)
	runTests(t, p) //nolint:errcheck // elastic-package detects errors by the results slice.
	if opt.TestWork {
		return true, nil
	}
	if err := cleanUp(context.WithoutCancel(ctx), pkgInfo, t); err != nil {
		t.Log("cleanup: ", err)
	}
	return true, nil
}

// resolveLatestVersion loads the package manifest and queries the registry to determine
// whether the local package is the latest published version.
// Returns a nil manifest when pkgInfo has no root (explicit --dir path).
func resolveLatestVersion(pkgInfo packageInfo, eprBaseURL string, prof *profile.Profile) (manifest *packages.PackageManifest, latestEPRVersion string, isLatestVersion bool, err error) {
	isLatestVersion = true
	if pkgInfo.root == "" {
		return nil, "", isLatestVersion, nil
	}
	manifest, err = packages.ReadPackageManifestFromPackageRoot(pkgInfo.root)
	if err != nil {
		return nil, "", false, err
	}
	revisions, err := revisionsFromRegistry(eprBaseURL, prof, manifest.Name)
	if err != nil {
		return nil, "", false, err
	}
	if len(revisions) > 0 && pkgInfo.currSemver != nil {
		latestEPRVersion = revisions[len(revisions)-1].Version
		latestSemver, err := semver.NewVersion(latestEPRVersion)
		if err != nil {
			return nil, "", false, fmt.Errorf("failed to parse latest epr version %q: %w", latestEPRVersion, err)
		}
		if latestSemver.GreaterThanEqual(pkgInfo.currSemver) {
			isLatestVersion = false
		}
	}
	return manifest, latestEPRVersion, isLatestVersion, nil
}

func newT(opt Options, dst *[]testrunner.TestResult, w io.Writer) (*T, error) {
	t := &T{
		pkg:     opt.Package,
		verbose: opt.Verbose,

		passthrough: w, out: w,
		results: dst,

		deployedService:      make(map[string]servicedeployer.DeployedService),
		runningStack:         make(map[string]*runningStack),
		installedAgents:      make(map[string]*installedAgent),
		installedDataStreams: make(map[string]struct{}),
		installedPipelines:   make(map[string]installedPipelines),
		registryPackages:     make(map[string][]registryPackage),
		registryPackageRoots: make(map[string]string),
	}
	if opt.RunPattern != "" {
		var err error
		t.run, err = regexp.Compile(opt.RunPattern)
		if err != nil {
			return nil, err
		}
	}
	return t, nil
}

func cleanUp(ctx context.Context, pkgInfo packageInfo, t *T) error {
	// We most likely have only one stack, but just iterate over
	// all if there is more than one. What could possibly go wrong?
	// If this _is_ problematic, we'll need to record the stack that
	// was used for each item when it's created.
	var errs []error
	for prof, stk := range t.runningStack {
		for _, pipe := range t.installedPipelines {
			if err := ingest.UninstallPipelines(ctx, stk.es.API, pipe.pipes); err != nil {
				errs = append(errs, fmt.Errorf("uninstalling pipelines: %w", err))
			}
		}

		for _, srv := range t.deployedService {
			if err := srv.TearDown(ctx); err != nil {
				errs = append(errs, fmt.Errorf("tearing down service: %w", err))
			}
		}

		for ds := range t.installedDataStreams {
			_, err := stk.es.Indices.DeleteDataStream([]string{ds},
				stk.es.Indices.DeleteDataStream.WithContext(ctx),
			)
			if err != nil {
				errs = append(errs, fmt.Errorf("deleting data stream %s: %w", ds, err))
			}
		}

		for _, installed := range t.installedAgents {
			if err := stk.kibana.RemoveAgent(ctx, installed.enrolled); err != nil {
				errs = append(errs, fmt.Errorf("removing agent: %w", err))
			}
			if err := installed.deployed.TearDown(ctx); err != nil {
				errs = append(errs, fmt.Errorf("tearing down agent deployer: %w", err))
			}
			if err := deletePolicies(ctx, stk.kibana, installed); err != nil {
				errs = append(errs, fmt.Errorf("deleting policies: %w", err))
			}
		}

		for _, pkg := range t.registryPackages[prof] {
			_, err := stk.kibana.RemovePackage(ctx, pkg.name, pkg.version)
			if err != nil && !strings.Contains(err.Error(), "status code = 404") && !strings.Contains(err.Error(), "is not installed") {
				errs = append(errs, fmt.Errorf("removing registry package %s-%s: %w", pkg.name, pkg.version, err))
			}
		}

		m := resources.NewManager()
		m.RegisterProvider(resources.DefaultKibanaProviderName, &resources.KibanaProvider{Client: stk.kibana})
		_, err := m.ApplyCtx(ctx, resources.Resources{&resources.FleetPackage{
			PackageRoot: pkgInfo.root,
			Absent:      true, // uninstall only — no bundling takes place, so no resolver is needed
			Force:       true,
		}})
		if err != nil && !strings.Contains(err.Error(), "is not installed") {
			errs = append(errs, err)
		}

		if stk.external {
			continue
		}
		if err := stk.provider.TearDown(ctx, stack.Options{Profile: stk.profile}); err != nil {
			errs = append(errs, fmt.Errorf("tearing down stack provider: %w", err))
		}
	}
	return errors.Join(errs...)
}

func scripts(dir string) ([]string, error) {
	if dir == "" {
		return nil, nil
	}
	fi, err := os.Stat(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("stat directory failed (path: %s): %w", dir, err)
	}
	if !fi.IsDir() {
		return nil, fmt.Errorf("data stream must be a directory (path: %s)", dir)
	}
	ent, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	if len(ent) == 0 {
		return nil, nil
	}
	return []string{dir}, nil
}

func clearStdStreams(ts *testscript.TestScript) {
	fmt.Fprint(ts.Stdout(), "")
	fmt.Fprint(ts.Stderr(), "")
}

func datastreams(streams []string, root string) ([]string, error) {
	if len(streams) == 0 {
		p := filepath.Join(root, "data_stream")
		fi, err := os.Stat(p)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				err = nil
			}
			return nil, err
		}
		if !fi.IsDir() {
			return nil, fmt.Errorf("data_stream must be a directory (path: %s)", p)
		}
		d, err := os.Open(p)
		if err != nil {
			return nil, err
		}
		defer d.Close()
		streams, err = d.Readdirnames(-1)
		if err != nil {
			return nil, err
		}
	}
	for i, ds := range streams {
		ds = strings.TrimSpace(ds)
		p := filepath.Join(root, "data_stream", ds)
		fi, err := os.Stat(p)
		if err != nil {
			return nil, fmt.Errorf("stat directory failed (path: %s): %w", p, err)
		}
		if !fi.IsDir() {
			return nil, fmt.Errorf("data stream must be a directory (path: %s)", p)
		}
		ent, err := os.ReadDir(p)
		if err != nil {
			return nil, err
		}
		if len(ent) == 0 {
			continue
		}
		streams[i] = ds
	}
	return streams, nil
}

func runTests(t *T, p testscript.Params) (err error) {
	defer func() {
		switch r := recover().(type) {
		case nil:
		case error:
			switch {
			case errors.Is(r, skipRun):
			default:
				err = r
			}
		default:
			panic(r)
		}
	}()

	testscript.RunT(t, p)
	if t.failed.Load() {
		return failedRun
	}
	return nil
}

var (
	failedRun = errors.New("failed run") //nolint:staticcheck // testscript convention: these sentinel errors are accessed by name, not via errors.Is
	skipRun   = errors.New("skip")       //nolint:staticcheck // testscript convention
)

// T implements testscript.T and is used in the call to testscript.Run
type T struct {
	pkg, dataStream string

	run           *regexp.Regexp
	verbose       bool
	stdinTempFile string
	failed        atomic.Bool

	passthrough, out io.Writer

	current testrunner.TestResult
	results *[]testrunner.TestResult

	// stack registries
	deployedService      map[string]servicedeployer.DeployedService
	runningStack         map[string]*runningStack
	installedAgents      map[string]*installedAgent
	installedDataStreams map[string]struct{}
	installedPipelines   map[string]installedPipelines
	registryPackages     map[string][]registryPackage
	registryPackageRoots map[string]string
}

// clearRegistries prevents tests within a directory from communicating
// with each other. This is required because we need a way to share the
// registries with the environment in order to do the clean-up.
func (t *T) clearRegistries() {
	clear(t.installedPipelines)
	clear(t.deployedService)
	clear(t.installedDataStreams)
	clear(t.installedAgents)
	clear(t.runningStack)
	clear(t.registryPackages)
	clear(t.registryPackageRoots)
}

func (t *T) Skip(is ...any) {
	t.current.Skipped = &testrunner.SkipConfig{Reason: fmt.Sprint(is...)}
	panic(skipRun)
}

func (t *T) Fatal(is ...any) {
	t.current.FailureMsg = fmt.Sprint(is...)
	t.Log(is...)
	t.FailNow()
}

func (t *T) Parallel() {
	// Not supported.
}

func (t *T) Log(is ...any) {
	msg := fmt.Sprint(is...)
	if t.stdinTempFile != "" {
		msg = strings.ReplaceAll(msg, t.stdinTempFile, "<stdin>")
	}
	if !strings.HasSuffix(msg, "\n") {
		msg += "\n"
	}
	if t.out == nil {
		t.out = os.Stdout
	}
	fmt.Fprint(t.out, msg)
}

func (t *T) FailNow() {
	panic(failedRun)
}

func (t *T) Run(name string, f func(t testscript.T)) {
	if t.run != nil && !t.run.MatchString(name) {
		return
	}
	t.current = testrunner.TestResult{
		Name:       name,
		Package:    t.pkg,
		DataStream: t.dataStream,
		TestType:   "script",
	}
	var buf bytes.Buffer
	t.out = io.MultiWriter(t.passthrough, &buf)
	start := time.Now()
	defer func() {
		switch err := recover(); err {
		case nil:
		case skipRun:
			t.Log("SKIPPED ", name)
		case failedRun:
			t.Log("FAILED ", name)
			t.current.FailureDetails = buf.String()
			if t.current.FailureMsg == "" {
				// A builtin failed us, so the Failure message
				// was not set.
				t.current.FailureMsg = reason(name, t.current.FailureDetails)
			}
			t.out = t.passthrough
			t.failed.Store(true)
		default:
			panic(fmt.Errorf("unexpected panic: %v [%T]", err, err))
		}
		t.current.TimeElapsed = time.Since(start)
		*t.results = append(*t.results, t.current)
	}()
	t.Log("RUN ", name)
	t.clearRegistries()
	f(t)
}

func reason(name, details string) string {
	sc := bufio.NewScanner(strings.NewReader(details))
	sep := string(filepath.Separator)
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "FAIL: ") {
			idx := strings.Index(line, sep+name+".txt:")
			if idx > 0 {
				return line[idx+len(sep):]
			}
			idx = strings.Index(line, sep+name+".txtar:")
			if idx > 0 {
				return line[idx+len(sep):]
			}
			// This should never be reached.
			return strings.TrimPrefix(line, "FAIL: ")
		}
	}
	return "failed for unknown reason"
}

func (t *T) Verbose() bool {
	return t.verbose
}

func decoratedWith(msg string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", msg, err)
}

func match(ts *testscript.TestScript, neg bool, args []string) {
	if len(args) != 2 {
		ts.Fatalf("usage: match_file pattern_file data")
	}
	pattern, err := os.ReadFile(ts.MkAbs(args[0]))
	ts.Check(decoratedWith("read pattern file", err))
	data, err := os.ReadFile(ts.MkAbs(args[1]))
	ts.Check(decoratedWith("read data file", err))
	// txtar files always end with a \n, so remove it.
	pattern = bytes.TrimRight(pattern, "\n")
	re, err := regexp.Compile("(?m)" + string(pattern))
	ts.Check(err)

	if neg {
		if re.Match(data) {
			ts.Logf("[match_file]\n%s\n", data)
			ts.Fatalf("unexpected match for %#q found in match_file: %s\n", pattern, re.Find(data))
		}
	} else {
		if !re.Match(data) {
			ts.Logf("[match_file]\n%s\n", data)
			ts.Fatalf("no match for %#q found in match_file", pattern)
		}
	}
}

// sleep waits for a specified duration.
func sleep(ts *testscript.TestScript, neg bool, args []string) {
	if neg {
		ts.Fatalf("unsupported: ! sleep")
	}
	if len(args) != 1 {
		ts.Fatalf("usage: sleep duration")
	}
	fmt.Println("sleep", args[0])
	d, err := time.ParseDuration(args[0])
	ts.Check(err)
	time.Sleep(d)
}

// date is the unix date command rendering the time in RFC3339, optionally
// storing the time into a named environment variable.
func date(ts *testscript.TestScript, neg bool, args []string) {
	if neg {
		ts.Fatalf("unsupported: ! date")
	}
	if len(args) != 0 && len(args) != 1 {
		ts.Fatalf("usage: date [<ENV_VAR_NAME>]")
	}
	t := time.Now().Format(time.RFC3339Nano)
	if len(args) == 1 {
		ts.Setenv(args[0], t)
	}
	_, err := fmt.Fprintln(ts.Stdout(), t)
	ts.Check(err)
}

func get(ts *testscript.TestScript, neg bool, args []string) {
	flg := flag.NewFlagSet("get", flag.ContinueOnError)
	jsonData := flg.Bool("json", false, "data from GET is JSON")
	ts.Check(flg.Parse(args))
	if flg.NArg() != 1 {
		ts.Fatalf("usage: GET [-json] <url>")
	}
	cli := http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}
	resp, err := cli.Get(flg.Arg(0))
	if neg {
		ts.Check(err)
	}
	var buf bytes.Buffer
	_, err = io.Copy(&buf, resp.Body)
	if neg {
		ts.Check(err)
	}
	err = resp.Body.Close()
	if neg {
		ts.Check(err)
	}
	if *jsonData {
		var dst bytes.Buffer
		err = json.Indent(&dst, buf.Bytes(), "", "\t")
		if neg {
			ts.Check(err)
		}
		buf = dst
	}
	ts.Stdout().Write(buf.Bytes()) //nolint:errcheck // testscript stdout is an in-memory buffer; write errors are not actionable
	if !bytes.HasSuffix(buf.Bytes(), []byte{'\n'}) {
		fmt.Fprintln(ts.Stdout())
	}
	if neg {
		ts.Fatalf("get: unexpected success")
	}
}

func post(ts *testscript.TestScript, neg bool, args []string) {
	flg := flag.NewFlagSet("post", flag.ContinueOnError)
	jsonData := flg.Bool("json", false, "response data from POST is JSON")
	content := flg.String("content", "", "data content-type")
	ts.Check(flg.Parse(args))
	if flg.NArg() != 2 {
		ts.Fatalf("usage: POST [-json] [-content <content-type>] <body-path> <url>")
	}
	f, err := os.Open(flg.Arg(0))
	if neg {
		ts.Check(err)
	}
	defer f.Close()
	cli := http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}
	resp, err := cli.Post(flg.Arg(1), *content, f)
	if neg {
		ts.Check(err)
	}
	var buf bytes.Buffer
	_, err = io.Copy(&buf, resp.Body)
	if neg {
		ts.Check(err)
	}
	err = resp.Body.Close()
	if neg {
		ts.Check(err)
	}
	if *jsonData {
		var dst bytes.Buffer
		err = json.Indent(&dst, buf.Bytes(), "", "\t")
		if neg {
			ts.Check(err)
		}
		buf = dst
	}
	ts.Stdout().Write(buf.Bytes()) //nolint:errcheck // testscript stdout is an in-memory buffer; write errors are not actionable
	if !bytes.HasSuffix(buf.Bytes(), []byte{'\n'}) {
		fmt.Fprintln(ts.Stdout())
	}
	if neg {
		ts.Fatalf("get: unexpected success")
	}
}

func expandTilde(path string) (string, error) {
	path, ok := strings.CutPrefix(path, "~/")
	if !ok {
		return path, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, path), nil
}

type printer struct {
	stdout, stderr io.Writer
}

func (p printer) Print(i ...interface{})                 { fmt.Fprint(p.stdout, i...) }
func (p printer) Println(i ...interface{})               { fmt.Fprintln(p.stdout, i...) }
func (p printer) Printf(format string, i ...interface{}) { fmt.Fprintf(p.stdout, format, i...) }
