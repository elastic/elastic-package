// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package script

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/rogpeppe/go-internal/testscript"

	"github.com/elastic/elastic-package/internal/elasticsearch"
	"github.com/elastic/elastic-package/internal/kibana"
	"github.com/elastic/elastic-package/internal/profile"
	"github.com/elastic/elastic-package/internal/stack"
	"github.com/elastic/elastic-package/internal/testrunner/runners/system"
)

// stackUp brings up a stack in the same way that `elastic-package stack up -d` does.
func stackUp(ts *testscript.TestScript, neg bool, args []string) {
	clearStdStreams(ts)

	if neg {
		ts.Fatalf("unsupported: ! stack_up")
	}

	stacks, ok := ts.Value(runningStackTag{}).(map[string]*runningStack)
	if !ok {
		ts.Fatalf("no active stacks registry")
	}

	flg := flag.NewFlagSet("up", flag.ContinueOnError)
	profName := flg.String("profile", "default", "profile name")
	provName := flg.String("provider", "compose", "provider name")
	timeout := flg.Duration("timeout", 0, "timeout (zero or lower indicates no timeout)")
	ts.Check(flg.Parse(args))
	if flg.NArg() != 1 {
		ts.Fatalf("usage: stack_up [-profile <profile>] [-provider <provider>] [-timeout <duration>] <stack-version>")
	}
	version := flg.Arg(0)

	ctx := context.Background()
	if *timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, *timeout)
		defer cancel()
	}

	prof, err := profile.LoadProfileFrom(ts.MkAbs("profiles"), *profName)
	ts.Check(decoratedWith("loading profile", err))
	prov, err := stack.BuildProvider(*provName, prof)
	ts.Check(decoratedWith("getting build provider", err))
	ts.Check(decoratedWith("booting stack", prov.BootUp(ctx, stack.Options{
		DaemonMode:   true,
		StackVersion: version,
		Services:     nil, // TODO
		Profile:      prof,
		Printer:      printer{stdout: ts.Stdout(), stderr: ts.Stderr()},
	})))

	cfg, err := stack.LoadConfig(prof)
	ts.Check(decoratedWith("loading config", err))

	es, err := stack.NewElasticsearchClientFromProfile(prof, elasticsearch.OptionWithCertificateAuthority(cfg.CACertFile))
	ts.Check(decoratedWith("making elasticsearch client", err))
	ts.Check(decoratedWith("checking cluster health", es.CheckHealth(ctx)))

	kibana, err := stack.NewKibanaClientFromProfile(prof, kibana.CertificateAuthority(cfg.CACertFile))
	ts.Check(decoratedWith("making kibana client", err))

	stacks[*profName] = &runningStack{
		version:  version,
		profile:  prof,
		provider: prov,
		config:   cfg,
		es:       es,
		kibana:   kibana,
	}
}

// stackDown takes down a stack in the same way that `elastic-package stack down` does.
func stackDown(ts *testscript.TestScript, neg bool, args []string) {
	clearStdStreams(ts)

	stacks, ok := ts.Value(runningStackTag{}).(map[string]*runningStack)
	if !ok {
		if neg {
			fmt.Fprintf(ts.Stderr(), "no active stacks registry")
			return
		}
		ts.Fatalf("no active stacks registry")
	}

	flg := flag.NewFlagSet("down", flag.ContinueOnError)
	profName := flg.String("profile", "default", "profile name")
	timeout := flg.Duration("timeout", 0, "timeout (zero or lower indicates no timeout)")
	ts.Check(flg.Parse(args))
	if flg.NArg() != 0 {
		ts.Fatalf("usage: stack_down [-profile <profile>] [-provider <provider>] [-timeout <duration>]")
	}
	stk, ok := stacks[*profName]
	if !ok {
		if neg {
			fmt.Fprintf(ts.Stderr(), "no running stack for %s", *profName)
			return
		}
		ts.Fatalf("no running stack for %s", *profName)
	}
	if stk.external {
		if neg {
			fmt.Fprintf(ts.Stderr(), "cannot take down externally run stack %s", *profName)
			return
		}
		ts.Fatalf("cannot take down externally run stack %s", *profName)
	}

	ctx := context.Background()
	if *timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, *timeout)
		defer cancel()
	}

	delete(stacks, *profName)

	ts.Check(decoratedWith("tearing down stack", stk.provider.TearDown(ctx, stack.Options{
		Profile: stk.profile,
		Printer: printer{stdout: ts.Stdout(), stderr: ts.Stderr()},
	})))
}

// useStack registers a running stack for use in the script.
func useStack(ts *testscript.TestScript, neg bool, args []string) {
	clearStdStreams(ts)

	if neg {
		ts.Fatalf("unsupported: ! use_stack")
	}

	stacks, ok := ts.Value(runningStackTag{}).(map[string]*runningStack)
	if !ok {
		ts.Fatalf("no active stacks registry")
	}

	flg := flag.NewFlagSet("use", flag.ContinueOnError)
	profPath := flg.String("profile", "~/.elastic-package/profiles/default", "profile path")
	timeout := flg.Duration("timeout", 0, "timeout (zero or lower indicates no timeout)")
	ts.Check(flg.Parse(args))
	if flg.NArg() != 0 {
		ts.Fatalf("usage: use_stack [-profile <profile>] [-timeout <duration>]")
	}
	if _, ok = stacks[*profPath]; ok {
		// Already registered, so we are done.
		return
	}

	path, err := expandTilde(*profPath)
	ts.Check(decoratedWith("getting home directory", err))

	ctx := context.Background()
	if *timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, *timeout)
		defer cancel()
	}

	dir := filepath.Dir(path)
	base := filepath.Base(path)
	prof, err := profile.LoadProfileFrom(ts.MkAbs(dir), base)
	ts.Check(decoratedWith("loading profile", err))
	cfg, err := stack.LoadConfig(prof)
	ts.Check(decoratedWith("loading config", err))
	provName := stack.DefaultProvider
	if cfg.Provider != "" {
		provName = cfg.Provider
	}
	prov, err := stack.BuildProvider(provName, prof)
	ts.Check(decoratedWith("getting build provider", err))

	es, err := stack.NewElasticsearchClientFromProfile(prof, elasticsearch.OptionWithCertificateAuthority(cfg.CACertFile))
	ts.Check(decoratedWith("making elasticsearch client", err))
	ts.Check(decoratedWith("checking cluster health", es.CheckHealth(ctx)))

	kibana, err := stack.NewKibanaClientFromProfile(prof, kibana.CertificateAuthority(cfg.CACertFile))
	ts.Check(decoratedWith("making kibana client", err))
	vi, err := kibana.Version()
	ts.Check(decoratedWith("getting kibana version", err))

	stacks[*profPath] = &runningStack{
		version:  vi.Version(),
		profile:  prof,
		provider: prov,
		config:   cfg,
		external: true,
		es:       es,
		kibana:   kibana,
	}
	msg, err := json.MarshalIndent(cfg, "", "\t")
	ts.Check(decoratedWith("marshaling config", err))
	fmt.Fprintf(ts.Stdout(), "%s\n", msg)
}

// getDocs performs a search on the current data stream or a named data stream
// and prints the results.
func getDocs(ts *testscript.TestScript, neg bool, args []string) {
	clearStdStreams(ts)

	if neg {
		ts.Fatalf("unsupported: ! get_docs")
	}

	stacks, ok := ts.Value(runningStackTag{}).(map[string]*runningStack)
	if !ok {
		ts.Fatalf("no active stacks registry")
	}

	flg := flag.NewFlagSet("get_docs", flag.ContinueOnError)
	profName := flg.String("profile", "default", "profile name")
	want := flg.Int("want", -1, "number of events expected (negative indicates any positive number)")
	querySize := flg.Int("size", 500, "profile name")
	confirmDuration := flg.Duration("confirm", 4*time.Second, "time to ensure hits do not exceed want count (zero or lower indicates no wait)")
	timeout := flg.Duration("timeout", time.Minute, "timeout (zero or lower indicates no timeout)")
	ts.Check(flg.Parse(args))
	if flg.NArg() != 0 && flg.NArg() != 1 {
		ts.Fatalf("usage: get_docs [-profile <profile>] [-timeout <duration>] [<data_stream>]")
	}

	ds := ts.Getenv("DATA_STREAM")
	if flg.NArg() == 1 {
		ds = flg.Arg(0)
	}
	if ds == "" {
		ts.Fatalf("no data stream specified")
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

	confirmed := false
	var body bytes.Buffer
	for {
		ts.Check(decoratedWith("performing search", ctx.Err()))

		resp, err := stk.es.Search(
			stk.es.Search.WithContext(ctx),
			stk.es.Search.WithIndex(ds),
			stk.es.Search.WithSort("@timestamp:asc"),
			stk.es.Search.WithSize(*querySize),
			stk.es.Search.WithSource("true"),
			stk.es.Search.WithBody(strings.NewReader(system.FieldsQuery)),
			stk.es.Search.WithIgnoreUnavailable(true),
		)
		resp.String()
		ts.Check(decoratedWith("performing search", err))
		body.Reset()
		_, err = io.Copy(&body, resp.Body)
		ts.Check(decoratedWith("reading search result", err))
		resp.Body.Close()

		if resp.StatusCode == http.StatusServiceUnavailable && bytes.Contains(body.Bytes(), []byte("no_shard_available_action_exception")) {
			// Index is being created, but no shards are available yet.
			// See https://github.com/elastic/elasticsearch/issues/65846
			time.Sleep(time.Second)
			continue
		}
		if resp.StatusCode >= 300 {
			ts.Fatalf("failed to get docs from data stream %s: %s", ds, body.Bytes())
		}

		var res system.FieldsQueryResult
		ts.Check(decoratedWith("unmarshaling result", json.Unmarshal(body.Bytes(), &res)))

		n := res.Hits.Total.Value
		if n < *want {
			time.Sleep(time.Second)
			continue
		}
		if n != 0 && *want < 0 {
			break
		}
		if n > *want && *want >= 0 {
			break
		}
		if n == *want {
			if confirmed || *confirmDuration == 0 {
				break
			}
			time.Sleep(*confirmDuration)
			confirmed = true
		}
		time.Sleep(time.Second)
	}
	fmt.Fprintf(ts.Stdout(), "%s\n", body.Bytes())
}

// dumpLogs copies logs to a directory within the work directory.
func dumpLogs(ts *testscript.TestScript, neg bool, args []string) {
	clearStdStreams(ts)

	if neg {
		ts.Fatalf("unsupported: ! dump_logs")
	}

	stacks, ok := ts.Value(runningStackTag{}).(map[string]*runningStack)
	if !ok {
		ts.Fatalf("no active stacks registry")
	}

	flg := flag.NewFlagSet("down", flag.ContinueOnError)
	profName := flg.String("profile", "default", "profile name")
	snce := flg.String("since", "", "get logs since this time (RFC3339)")
	timeout := flg.Duration("timeout", 0, "timeout (zero or lower indicates no timeout)")
	ts.Check(flg.Parse(args))
	if flg.NArg() != 0 && flg.NArg() != 1 {
		ts.Fatalf("usage: dump_logs [-profile <profile>] [-provider <provider>] [-timeout <duration>] [-since <RFC3339 time>] [<dirpath>]")
	}
	stk, ok := stacks[*profName]
	if !ok {
		ts.Fatalf("no running stack for %s", *profName)
	}

	dir := "."
	if flg.NArg() == 1 {
		dir = filepath.Clean(flg.Arg(0))
	}
	if dir == "." {
		dir = "logs"
	}
	_, err := os.Stat(ts.MkAbs(dir))
	if err == nil {
		ts.Fatalf("%q exists", dir)
	}

	// Make the target directory safe, and ensure that
	// it its parent is present and within $WORK, and
	// the actual target is absent.
	r, err := os.OpenRoot(ts.MkAbs("."))
	ts.Check(decoratedWith("making root jail", err))
	ts.Check(decoratedWith("making logs destination", r.MkdirAll(dir, 0o700)))
	ts.Check(decoratedWith("cleaning logs destination", r.Remove(dir)))

	// This is necessary to allow writing a log directory to $WORK,
	// something that is otherwise impossible because stack.Dump
	// doesn't just write to stack.DumpOptions.Output, but to
	// filepath.Join(options.Output, "logs"), _and_ deletes all
	// of options.Output. So a path of "." deletes $WORK despite
	// only needing to, at most, delete $WORK/logs. ¯\_(ツ)_/¯
	tmp, err := os.MkdirTemp(ts.MkAbs("."), "*")
	ts.Check(decoratedWith("making temporary logs directory", err))

	var since time.Time
	if *snce != "" {
		var err error
		since, err = time.Parse(time.RFC3339Nano, *snce)
		ts.Check(decoratedWith("parsing since flag", err))
	}

	ctx := context.Background()
	if *timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, *timeout)
		defer cancel()
	}

	_, err = stack.Dump(ctx, stack.DumpOptions{
		Output:  tmp,
		Profile: stk.profile,
		Since:   since,
	})
	ts.Check(decoratedWith("dumping agent logs", err))
	ts.Check(decoratedWith("moving logs", os.Rename(filepath.Join(tmp, "logs"), filepath.Join(ts.MkAbs(dir)))))
	ts.Check(decoratedWith("removing temporary logs target", os.Remove(tmp)))

	// Collect all internal logs into canonical locations in temporal order.
	for _, l := range []string{
		"elastic-agent-internal",
		"fleet-server-internal",
	} {
		g, err := filepath.Glob(filepath.Join(ts.MkAbs(dir), l, "*"))
		ts.Check(decoratedWith(fmt.Sprintf("collecting %s logs", l), err))
		type message struct {
			ts   time.Time
			json json.RawMessage
		}
		var log []message
		for _, p := range g {
			f, err := os.Open(p)
			ts.Check(decoratedWith(fmt.Sprintf("reading %s logs", l), err))
			dec := json.NewDecoder(f)
			for {
				var line message
				err := dec.Decode(&line.json)
				if err == io.EOF {
					break
				}
				ts.Check(decoratedWith(fmt.Sprintf("unmarshaling logs in %s", p), err))
				var probe struct {
					Timestamp time.Time `json:"@timestamp"`
				}
				ts.Check(decoratedWith(fmt.Sprintf("unmarshaling timestamp in %s", p), json.Unmarshal(line.json, &probe)))
				line.ts = probe.Timestamp
				log = append(log, line)
			}
			ts.Check(decoratedWith(fmt.Sprintf("closing %s", p), f.Close()))
		}
		slices.SortStableFunc(log, func(a, b message) int {
			return int(a.ts.Sub(b.ts))
		})
		o, err := os.Create(filepath.Join(ts.MkAbs(dir), l, "elastic-agent-all.ndjson"))
		ts.Check(decoratedWith(fmt.Sprintf("creating %s logs collection", l), err))
		for _, line := range log {
			_, err := o.Write(line.json)
			ts.Check(decoratedWith(fmt.Sprintf("writing %s logs collection", l), err))
			_, err = o.WriteString("\n")
			ts.Check(decoratedWith(fmt.Sprintf("writing new line to %s logs collection", l), err))
		}
		ts.Check(decoratedWith(fmt.Sprintf("syncing %s logs collection", l), o.Sync()))
		ts.Check(decoratedWith(fmt.Sprintf("closing %s logs collection", l), o.Close()))
	}
}

type runningStackTag struct{}

type runningStack struct {
	version  string
	profile  *profile.Profile
	provider stack.Provider
	config   stack.Config
	external bool

	es     *elasticsearch.Client
	kibana *kibana.Client
}
