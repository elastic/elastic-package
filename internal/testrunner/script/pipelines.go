// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package script

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/rogpeppe/go-internal/testscript"

	"github.com/elastic/elastic-package/internal/elasticsearch/ingest"
)

// installPipelines installs a data stream's pipelines.
func installPipelines(ts *testscript.TestScript, neg bool, args []string) {
	clearStdStreams(ts)

	if neg {
		ts.Fatalf("unsupported: ! install_pipelines")
	}

	stacks, ok := ts.Value(runningStackTag{}).(map[string]*runningStack)
	if !ok {
		ts.Fatalf("no active stacks registry")
	}

	flg := flag.NewFlagSet("install", flag.ContinueOnError)
	profName := flg.String("profile", "default", "profile name")
	timeout := flg.Duration("timeout", 0, "timeout (zero or lower indicates no timeout)")
	ts.Check(flg.Parse(args))
	if flg.NArg() != 1 {
		ts.Fatalf("usage: install_pipelines [-profile <profile>] [-timeout <duration>] <path_to_data_stream>")
	}

	name := flg.Arg(0)
	path := ts.MkAbs(name)

	_, err := os.Stat(filepath.Join(path, filepath.FromSlash("elasticsearch/ingest_pipeline")))
	ts.Check(err)

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

	nonce := time.Now().UnixNano()
	pipes, err := ingest.LoadIngestPipelineFiles(path, nonce)
	ts.Check(decoratedWith("loading pipelines", err))

	ts.Check(decoratedWith("installing pipelines", ingest.InstallPipelinesInElasticsearch(ctx, stk.es.API, pipes)))

	pipelines, ok := ts.Value(installedPipelinesTag{}).(map[string]installedPipelines)
	if !ok {
		ts.Fatalf("no installed pipelines registry")
	}
	pipelines[name] = installedPipelines{
		path:  path,
		nonce: nonce,
		pipes: pipes,
	}
	fmt.Fprintf(ts.Stdout(), "installed pipelines in %s with nonce %d", filepath.Base(path), nonce)
}

// simulate runs the simulate API endpoint using a data stream's pipelines.
func simulate(ts *testscript.TestScript, neg bool, args []string) {
	clearStdStreams(ts)

	if neg {
		ts.Fatalf("unsupported: ! simulate")
	}

	stacks, ok := ts.Value(runningStackTag{}).(map[string]*runningStack)
	if !ok {
		ts.Fatalf("no active stacks registry")
	}
	pipelines, ok := ts.Value(installedPipelinesTag{}).(map[string]installedPipelines)
	if !ok {
		ts.Fatalf("no installed pipelines registry")
	}

	flg := flag.NewFlagSet("uninstall", flag.ContinueOnError)
	profName := flg.String("profile", "default", "profile name")
	index := flg.String("index", "index-default", "simulate index name")
	timeout := flg.Duration("timeout", 0, "timeout (zero or lower indicates no timeout)")
	ts.Check(flg.Parse(args))
	if flg.NArg() != 3 {
		ts.Fatalf("usage: simulate [-profile <profile>] [-timeout <duration>] <path_to_data_stream> <pipeline> <path_to_data>")
	}

	name := flg.Arg(0)
	pipeline := flg.Arg(1)
	path := ts.MkAbs(flg.Arg(2))

	f, err := os.Open(path)
	ts.Check(err)
	defer f.Close()
	dec := json.NewDecoder(f)
	var events []json.RawMessage
	for {
		var e json.RawMessage
		err := dec.Decode(&e)
		if err == io.EOF {
			break
		}
		ts.Check(err)
		events = append(events, e)
	}

	installed, ok := pipelines[name]
	if !ok {
		ts.Fatalf("pipelines in %s are not installed", name)
	}
	pipeline = ingest.GetPipelineNameWithNonce(pipeline, installed.nonce)

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

	msg, err := ingest.SimulatePipeline(ctx, stk.es.API, pipeline, events, *index)
	ts.Check(decoratedWith("running simulate", err))

	for _, m := range msg {
		m, err := json.MarshalIndent(m, "", "\t")
		ts.Check(err)
		fmt.Fprintf(ts.Stdout(), "%s\n", m)
	}
}

// uninstallPipelines uninstalls a data stream's pipelines.
func uninstallPipelines(ts *testscript.TestScript, neg bool, args []string) {
	clearStdStreams(ts)

	if neg {
		ts.Fatalf("unsupported: ! uninstall_pipelines")
	}

	stacks, ok := ts.Value(runningStackTag{}).(map[string]*runningStack)
	if !ok {
		ts.Fatalf("no active stacks registry")
	}
	pipelines, ok := ts.Value(installedPipelinesTag{}).(map[string]installedPipelines)
	if !ok {
		ts.Fatalf("no installed pipelines registry")
	}

	flg := flag.NewFlagSet("uninstall", flag.ContinueOnError)
	profName := flg.String("profile", "default", "profile name")
	timeout := flg.Duration("timeout", 0, "timeout (zero or lower indicates no timeout)")
	ts.Check(flg.Parse(args))
	if flg.NArg() != 1 {
		ts.Fatalf("usage: uninstall_pipelines [-profile <profile>] [-timeout <duration>] <path_to_data_stream>")
	}

	name := flg.Arg(0)

	installed, ok := pipelines[name]
	if !ok {
		ts.Fatalf("pipelines in %s are not installed", name)
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

	ts.Check(decoratedWith("uninstall pipelines", ingest.UninstallPipelines(ctx, stk.es.API, installed.pipes)))

	delete(pipelines, name)
	fmt.Fprintf(ts.Stdout(), "uninstalled pipelines in %s", filepath.Base(installed.path))
}

type installedPipelinesTag struct{}

type installedPipelines struct {
	path  string
	nonce int64
	pipes []ingest.Pipeline
}
