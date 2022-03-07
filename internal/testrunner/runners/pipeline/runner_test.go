// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package pipeline

import (
	"bytes"
	"encoding/json"
	"flag"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/elastic/elastic-package/internal/fields"
	"github.com/elastic/elastic-package/internal/testrunner"
	"github.com/rogpeppe/go-internal/testscript"
	"github.com/stretchr/testify/require"
)

const (
	firstTestResult  = "first"
	secondTestResult = "second"
	thirdTestResult  = "third"

	emptyTestResult = ""
)

func TestStripEmptyTestResults(t *testing.T) {
	given := &testResult{
		events: []json.RawMessage{
			[]byte(firstTestResult),
			nil,
			nil,
			[]byte(emptyTestResult),
			[]byte(secondTestResult),
			nil,
			[]byte(thirdTestResult),
			nil,
		},
	}

	actual := stripEmptyTestResults(given)
	require.Len(t, actual.events, 4)
	require.Equal(t, actual.events[0], json.RawMessage(firstTestResult))
	require.Equal(t, actual.events[1], json.RawMessage(emptyTestResult))
	require.Equal(t, actual.events[2], json.RawMessage(secondTestResult))
	require.Equal(t, actual.events[3], json.RawMessage(thirdTestResult))
}

var update = flag.Bool("update", false, "update testscript output files")

func TestMain(m *testing.M) {
	os.Exit(testscript.RunMain(m, map[string]func() int{
		"runValidation": runValidation,
	}))
}

func TestScripts(t *testing.T) {
	t.Parallel()

	p := testscript.Params{
		Dir:           filepath.Join("testdata"),
		UpdateScripts: *update,
		Cmds: map[string]func(ts *testscript.TestScript, neg bool, args []string){
			"trimspace": trimSpace,
		},
	}
	testscript.Run(t, p)
}

// runValidation is a command abstraction of (*runner).runValidation working in
// the current directory  with no ingest pipelines and obtaining results from a
// golden file rather than a call to elasticsearch.
func runValidation() int {
	args := os.Args[1:]
	if len(args) != 5 {
		log.Fatalf("usage: runValidation <config-path> <datastream-path> <expected-path> <obtained-path> <generate:bool>")
	}

	generate, err := strconv.ParseBool(args[4])
	if err != nil {
		log.Fatalf("invalid generate parameter: %q", args[4])
	}
	r := runner{options: testrunner.TestOptions{
		TestFolder:         testrunner.TestFolder{Path: "."},
		GenerateTestResult: generate,
	}}

	wd, _ := os.Getwd()
	// Read config.
	b, err := os.ReadFile(args[0])
	if err != nil {
		log.Fatalf("failed to read config: %v %v", err, wd)
	}
	var config testConfig
	err = json.Unmarshal(b, &config)
	if err != nil {
		log.Fatalf("failed to unmarshal config: %v", err)
	}

	// Get path to golden file.
	testCaseFile := args[2]
	tc, err := r.loadTestCaseFile(testCaseFile)
	if err != nil {
		log.Fatalf("failed to read test case file: %v", err)
	}

	// Construct validator.
	dataStreamPath := args[1]
	fieldsValidator, err := fields.CreateValidatorForDataStream(dataStreamPath,
		fields.WithNumericKeywordFields(tc.config.NumericKeywordFields),
		fields.WithEnabledAllowedIPCheck(),
	)
	if err != nil {
		log.Fatalf("failed to read validator: %v", err)
	}

	// Get "obtained" result.
	b, err = os.ReadFile(args[3])
	if err != nil {
		log.Fatalf("failed to read result: %v", err)
	}
	var result struct {
		Events []json.RawMessage
	}
	err = jsonUnmarshalUsingNumber(b, &result) // Make sure we don't corrupt longs wider than 53 bits.
	if err != nil {
		log.Fatalf("failed to unmarshal result: %v", err)
	}

	err = r.verifyResults(testCaseFile, &config, &testResult{events: result.Events}, fieldsValidator)
	if err != nil {
		log.Fatalf("failed to verify: %v", err)
	}

	return 0
}

// trimspace is required to normalise txtar files with generated *-expected.json files
// since txtar cannot express files without a trailing newline and *-expected.json files
// never have a trailing newline.
func trimSpace(ts *testscript.TestScript, neg bool, args []string) {
	if neg {
		ts.Fatalf("unsupported: ! trimspace")
	}
	if len(args) < 1 {
		ts.Fatalf("usage: trimspace src...")
	}

	for _, arg := range args {
		src := ts.MkAbs(arg)
		info, err := os.Stat(src)
		ts.Check(err)
		data, err := ioutil.ReadFile(src)
		ts.Check(err)
		data = bytes.TrimSpace(data)
		ts.Check(ioutil.WriteFile(src, data, info.Mode()&0o777))
	}
}
