// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package script

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/rogpeppe/go-internal/testscript"

	"github.com/elastic/elastic-package/internal/elasticsearch"
)

var errPolicyNotFound = errors.New("not found")

func getPolicyCommand(ts *testscript.TestScript, neg bool, args []string) {
	clearStdStreams(ts)

	pkg := ts.Getenv("PKG")
	if pkg == "" {
		ts.Fatalf("PKG is not set")
	}
	ds := ts.Getenv("DATA_STREAM")
	if ds == "" {
		ts.Fatalf("DATA_STREAM is not set")
	}

	stacks, ok := ts.Value(runningStackTag{}).(map[string]*runningStack)
	if !ok {
		ts.Fatalf("no active stacks registry")
	}

	flg := flag.NewFlagSet("policies", flag.ContinueOnError)
	profName := flg.String("profile", "default", "profile name")
	timeout := flg.Duration("timeout", -1, "timeout (negative indicates single probe only, zero indicates no timeout)")
	ts.Check(flg.Parse(args))
	if flg.NArg() != 1 {
		ts.Fatalf("usage: get_policy [-profile <profile>] [-timeout <duration>] <policy_name>")
	}

	policyName := flg.Arg(0)

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

	if *timeout < 0 {
		// Single check.
		pol, err := getPolicy(ctx, stk.es.API, policyName)
		switch err {
		case nil:
			fmt.Fprint(ts.Stdout(), pol)
		case errPolicyNotFound:
			fmt.Fprint(ts.Stdout(), "not found")
		default:
			fmt.Fprint(ts.Stdout(), err)
		}
		return
	}

	for {
		// Check until found or timeout.
		pol, err := getPolicy(ctx, stk.es.API, policyName)
		switch err {
		case nil:
			fmt.Fprint(ts.Stdout(), pol)
			return
		case errPolicyNotFound:
			time.Sleep(time.Second)
			continue
		default:
			fmt.Fprint(ts.Stdout(), err)
			return
		}
	}
}

func getPolicy(ctx context.Context, cli *elasticsearch.API, name string) (string, error) {
	resp, err := cli.Indices.GetDataStream(cli.Indices.GetDataStream.WithContext(ctx))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var buf bytes.Buffer
	io.Copy(&buf, resp.Body)
	var body struct {
		DataStreams []json.RawMessage `json:"data_streams"`
	}
	err = json.Unmarshal(buf.Bytes(), &body)
	if err != nil {
		return "", err
	}
	var names []string
	for _, ds := range body.DataStreams {
		var probe struct {
			Name string `json:"name"`
		}
		err = json.Unmarshal(ds, &probe)
		if err != nil {
			return "", err
		}
		if name == "" {
			names = append(names, probe.Name)
			continue
		}
		if probe.Name == name {
			return string(ds), nil
		}
	}
	if names != nil {
		return strings.Join(names, "\n"), nil
	}
	return "", errPolicyNotFound
}
