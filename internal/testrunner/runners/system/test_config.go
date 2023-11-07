// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package system

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/aymerick/raymond"

	"github.com/elastic/go-ucfg"
	"github.com/elastic/go-ucfg/yaml"

	"github.com/elastic/elastic-package/internal/common"
	"github.com/elastic/elastic-package/internal/servicedeployer"
	"github.com/elastic/elastic-package/internal/testrunner"
)

var systemTestConfigFilePattern = regexp.MustCompile(`^test-([a-z0-9_.-]+)-config.yml$`)

type testConfig struct {
	testrunner.SkippableConfig `config:",inline"`

	Input               string        `config:"input"`
	PolicyTemplate      string        `config:"policy_template"` // Policy template associated with input. Required when multiple policy templates include the input being tested.
	Service             string        `config:"service"`
	ServiceNotifySignal string        `config:"service_notify_signal"` // Signal to send when the agent policy is applied.
	IgnoreServiceError  *bool         `config:"ignore_service_error"`
	WaitForDataTimeout  time.Duration `config:"wait_for_data_timeout"`

	Vars       common.MapStr `config:"vars"`
	DataStream struct {
		Vars common.MapStr `config:"vars"`
	} `config:"data_stream"`

	Assert struct {
		// Expected number of hits for a given test
		HitCount int `config:"hit_count"`
	} `config:"assert"`

	// NumericKeywordFields holds a list of fields that have keyword
	// type but can be ingested as numeric type.
	NumericKeywordFields []string `config:"numeric_keyword_fields"`

	Path               string `config:",ignore"` // Path of config file.
	ServiceVariantName string `config:",ignore"` // Name of test variant when using variants.yml.
}

func (t testConfig) Name() string {
	name := filepath.Base(t.Path)
	if matches := systemTestConfigFilePattern.FindStringSubmatch(name); len(matches) > 1 {
		name = matches[1]
	}

	var sb strings.Builder
	sb.WriteString(name)

	if t.ServiceVariantName != "" {
		sb.WriteString(" (variant: ")
		sb.WriteString(t.ServiceVariantName)
		sb.WriteString(")")
	}
	return sb.String()
}

func (t testConfig) MustIgnoreServiceError() bool {
	return t.IgnoreServiceError == nil || *t.IgnoreServiceError
}

func newConfig(configFilePath string, ctxt servicedeployer.ServiceContext, serviceVariantName string) (*testConfig, error) {
	data, err := os.ReadFile(configFilePath)
	if err != nil && errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("unable to find system test configuration file: %s: %w", configFilePath, err)
	}

	if err != nil {
		return nil, fmt.Errorf("could not load system test configuration file: %s: %w", configFilePath, err)
	}

	data, err = applyContext(data, ctxt)
	if err != nil {
		return nil, fmt.Errorf("could not apply context to test configuration file: %s: %w", configFilePath, err)
	}

	var c testConfig
	cfg, err := yaml.NewConfig(data, ucfg.PathSep("."))
	if err != nil {
		return nil, fmt.Errorf("unable to load system test configuration file: %s: %w", configFilePath, err)
	}
	if err := cfg.Unpack(&c); err != nil {
		return nil, fmt.Errorf("unable to unpack system test configuration file: %s: %w", configFilePath, err)
	}
	// Save path
	c.Path = configFilePath
	c.ServiceVariantName = serviceVariantName
	return &c, nil
}

func listConfigFiles(systemTestFolderPath string) (files []string, err error) {
	fHandle, err := os.Open(systemTestFolderPath)
	if err != nil {
		return nil, err
	}
	defer fHandle.Close()
	dirEntries, err := fHandle.Readdir(0)
	if err != nil {
		return nil, err
	}
	for _, entry := range dirEntries {
		if !entry.IsDir() && systemTestConfigFilePattern.MatchString(entry.Name()) {
			files = append(files, entry.Name())
		}
	}
	return files, nil
}

// applyContext takes the given system test configuration (data) and replaces any placeholder variables in
// it with values from the given context (ctxt). The context may be populated from various sources but usually the
// most interesting context values will be set by a ServiceDeployer in its SetUp method.
func applyContext(data []byte, ctxt servicedeployer.ServiceContext) ([]byte, error) {
	tmpl, err := raymond.Parse(string(data))
	if err != nil {
		return data, fmt.Errorf("parsing template body failed: %w", err)
	}
	tmpl.RegisterHelpers(ctxt.Aliases())

	result, err := tmpl.Exec(ctxt)
	if err != nil {
		return data, fmt.Errorf("could not render data with context: %w", err)
	}
	return []byte(result), nil
}
