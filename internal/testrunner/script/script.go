// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package script

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"sync/atomic"
	"time"

	"github.com/rogpeppe/go-internal/testscript"
	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/cobraext"
	"github.com/elastic/elastic-package/internal/configuration/locations"
	"github.com/elastic/elastic-package/internal/elasticsearch/ingest"
	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/elastic-package/internal/packages/changelog"
	"github.com/elastic/elastic-package/internal/resources"
	"github.com/elastic/elastic-package/internal/servicedeployer"
	"github.com/elastic/elastic-package/internal/stack"
)

func Run(dst io.Writer, cmd *cobra.Command, args []string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("could not find home: %w", err)
	}
	loc, err := locations.NewLocationManager()
	if err != nil {
		return err
	}
	work, err := cmd.Flags().GetBool(cobraext.WorkScriptTestFlagName)
	if err != nil {
		return err
	}
	workRoot := filepath.Join(home, filepath.FromSlash(".elastic-package/tmp/script_tests"))
	err = os.MkdirAll(workRoot, 0o700)
	if err != nil {
		return fmt.Errorf("could not make work space root: %w", err)
	}
	var workdirRoot string
	if work {
		// Only create a work root and pass it in if --work has been requested.
		// The behaviour of testscript is to set TestWork to true if the work
		// root is non-zero, so just let testscript put it where it wants in the
		// case that we have not requested work to be retained. This will be in
		// os.MkdirTemp(os.Getenv("GOTMPDIR"), "go-test-script") which on most
		// systems will be /tmp/go-test-script. However, due to… decisions, we
		// cannot operate in that directory…
		workdirRoot, err = os.MkdirTemp(workRoot, "*")
		if err != nil {
			return fmt.Errorf("could not make work space: %w", err)
		}
	} else {
		// … so set $GOTMPDIR to a location that we can work in.
		//
		// This is all obviously awful.
		err = os.Setenv("GOTMPDIR", workRoot)
		if err != nil {
			return fmt.Errorf("could not set temp dir var: %w", err)
		}
	}

	externalStack, err := cmd.Flags().GetBool(cobraext.ExternalStackFlagName)
	if err != nil {
		return err
	}
	run, err := cmd.Flags().GetString(cobraext.RunPatternFlagName)
	if err != nil {
		return err
	}
	verbose, err := cmd.Flags().GetCount(cobraext.VerboseFlagName)
	if err != nil {
		return err
	}
	verboseScript, err := cmd.Flags().GetBool(cobraext.VerboseScriptFlagName)
	if err != nil {
		return err
	}
	update, err := cmd.Flags().GetBool(cobraext.UpdateScriptTestArchiveFlagName)
	if err != nil {
		return err
	}
	cont, err := cmd.Flags().GetBool(cobraext.ContinueOnErrorFlagName)
	if err != nil {
		return err
	}

	dirs, err := scripts(cmd)
	if err != nil {
		return err
	}
	var pkgRoot, currVersion, prevVersion string
	if len(dirs) == 0 {
		var ok bool
		pkgRoot, ok, err = packages.FindPackageRoot()
		if err != nil {
			return fmt.Errorf("locating package root failed: %w", err)
		}
		if !ok {
			return errors.New("package root not found")
		}
		dirs, err = datastreams(cmd, pkgRoot)
		if err != nil {
			return err
		}
		if len(dirs) == 0 {
			return nil
		}
		revs, err := changelog.ReadChangelogFromPackageRoot(pkgRoot)
		if err != nil {
			return err
		}
		if len(revs) > 0 {
			currVersion = revs[0].Version
		}
		if len(revs) > 1 {
			prevVersion = revs[1].Version
		}
	}

	var stdinTempFile string
	t := &T{
		verbose:       verbose != 0 || verboseScript,
		stdinTempFile: stdinTempFile,

		out: dst,

		deployedService:      make(map[string]servicedeployer.DeployedService),
		runningStack:         make(map[string]*runningStack),
		installedAgents:      make(map[string]*installedAgent),
		installedDataStreams: make(map[string]struct{}),
		installedPipelines:   make(map[string]installedPipelines),
	}
	if run != "" {
		t.run, err = regexp.Compile(run)
		if err != nil {
			return nil
		}
	}
	var errs []error
	if pkgRoot != "" {
		t.Log("PKG ", filepath.Base(pkgRoot))
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	var n int
	for _, d := range dirs {
		scripts := d
		var dsRoot string
		if pkgRoot != "" {
			dsRoot = filepath.Join(pkgRoot, "data_stream", d)
			scripts = filepath.Join(dsRoot, filepath.FromSlash("_dev/test/scripts"))
		}
		_, err := os.Stat(scripts)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		n++
		p := testscript.Params{
			Dir:             scripts,
			WorkdirRoot:     workdirRoot,
			UpdateScripts:   update,
			ContinueOnError: cont,
			TestWork:        work,
			Cmds: map[string]func(ts *testscript.TestScript, neg bool, args []string){
				"sleep":                  sleep,
				"date":                   date,
				"GET":                    get,
				"POST":                   post,
				"stack_up":               stackUp,
				"use_stack":              useStack,
				"stack_down":             stackDown,
				"docker_up":              dockerUp,
				"docker_down":            dockerDown,
				"docker_signal":          dockerSignal,
				"docker_wait_exit":       dockerWaitExit,
				"install_pipelines":      installPipelines,
				"simulate":               simulate,
				"uninstall_pipelines":    uninstallPipelines,
				"install_agent":          installAgent,
				"add_package":            addPackage,
				"remove_package":         removePackage,
				"upgrade_package_latest": upgradePackageLatest,
				"add_package_zip":        addPackageZip,
				"remove_package_zip":     removePackageZip,
				"add_data_stream":        addDataStream,
				"remove_data_stream":     removeDataStream,
				"uninstall_agent":        uninstallAgent,
				"get_docs":               getDocs,
				"dump_logs":              dumpLogs,
				"match_file":             match,
				"get_policy":             getPolicyCommand,
			},
			Setup: func(e *testscript.Env) error {
				e.Setenv("CONFIG_ROOT", loc.RootDir())
				e.Setenv("CONFIG_PROFILES", loc.ProfileDir())
				e.Setenv("HOME", home)
				if pkgRoot != "" {
					e.Setenv("PKG", filepath.Base(pkgRoot))
					e.Setenv("PKG_ROOT", pkgRoot)
				}
				if currVersion != "" {
					e.Setenv("CURRENT_VERSION", currVersion)
				}
				if prevVersion != "" {
					e.Setenv("PREVIOUS_VERSION", prevVersion)
				}
				if dsRoot != "" {
					e.Setenv("DATA_STREAM", d)
					e.Setenv("DATA_STREAM_ROOT", dsRoot)
				}
				e.Values[deployedServiceTag{}] = t.deployedService
				e.Values[runningStackTag{}] = t.runningStack
				e.Values[installedAgentsTag{}] = t.installedAgents
				e.Values[installedDataStreamsTag{}] = t.installedDataStreams
				e.Values[installedPipelinesTag{}] = t.installedPipelines
				return nil
			},
			Condition: func(cond string) (bool, error) {
				switch cond {
				case "external_stack":
					return externalStack, nil
				default:
					return false, fmt.Errorf("unknown condition: %s", cond)
				}
			},
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
		err = runTests(t, p)
		if err != nil {
			errs = append(errs, err)
		}
		if work {
			continue
		}
		cleanUp(
			context.Background(), // Not the interrupt context.
			pkgRoot,
			t.deployedService,
			t.installedDataStreams,
			t.installedAgents,
			t.installedPipelines,
			t.runningStack,
		)
	}
	if n == 0 {
		t.Log("[no test files]")
	}
	return errors.Join(errs...)
}

func cleanUp(ctx context.Context, pkgRoot string, srvs map[string]servicedeployer.DeployedService, streams map[string]struct{}, agents map[string]*installedAgent, pipes map[string]installedPipelines, stacks map[string]*runningStack) {
	// We most likely have only one stack, but just iterate over
	// all if there is more than one. What could possibly go wrong?
	// If this _is_ problematic, we'll need to record the stack that
	// was used for each item when it's created.
	for _, stk := range stacks {
		for _, pipe := range pipes {
			ingest.UninstallPipelines(ctx, stk.es.API, pipe.pipes)
		}

		for _, srv := range srvs {
			srv.TearDown(ctx)
		}

		for ds := range streams {
			stk.es.Indices.DeleteDataStream([]string{ds},
				stk.es.Indices.DeleteDataStream.WithContext(ctx),
			)
		}

		for _, installed := range agents {
			stk.kibana.RemoveAgent(ctx, installed.enrolled)
			installed.deployed.TearDown(ctx)
			deletePolicies(ctx, stk.kibana, installed)
		}

		m := resources.NewManager()
		m.RegisterProvider(resources.DefaultKibanaProviderName, &resources.KibanaProvider{Client: stk.kibana})
		m.ApplyCtx(ctx, resources.Resources{&resources.FleetPackage{
			RootPath: pkgRoot,
			Absent:   true,
			Force:    true,
		}})

		if stk.external {
			continue
		}
		stk.provider.TearDown(ctx, stack.Options{Profile: stk.profile})
	}
}

func scripts(cmd *cobra.Command) ([]string, error) {
	dir, err := cmd.Flags().GetString(cobraext.ScriptsFlagName)
	if err != nil {
		return nil, err
	}
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

func datastreams(cmd *cobra.Command, root string) ([]string, error) {
	streams, err := cmd.Flags().GetStringSlice(cobraext.DataStreamsFlagName)
	if err != nil {
		return nil, cobraext.FlagParsingError(err, cobraext.DataStreamsFlagName)
	}
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
	//lint:ignore ST1012 This naming is conventional for testscript.
	failedRun = errors.New("failed run")
	//lint:ignore ST1012 This naming is conventional for testscript.
	skipRun = errors.New("skip")
)

// T implements testscript.T and is used in the call to testscript.Run
type T struct {
	run           *regexp.Regexp
	verbose       bool
	stdinTempFile string
	failed        atomic.Bool

	out io.Writer

	// stack registries
	deployedService      map[string]servicedeployer.DeployedService
	runningStack         map[string]*runningStack
	installedAgents      map[string]*installedAgent
	installedDataStreams map[string]struct{}
	installedPipelines   map[string]installedPipelines
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
}

func (t *T) Skip(is ...any) {
	panic(skipRun)
}

func (t *T) Fatal(is ...any) {
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
	defer func() {
		switch err := recover(); err {
		case nil:
		case skipRun:
			t.Log("SKIPPED ", name)
		case failedRun:
			t.Log("FAILED ", name)
			t.failed.Store(true)
		default:
			panic(fmt.Errorf("unexpected panic: %v [%T]", err, err))
		}
	}()
	t.Log("RUN ", name)
	t.clearRegistries()
	f(t)
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
	ts.Stdout().Write(buf.Bytes())
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
	ts.Stdout().Write(buf.Bytes())
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
