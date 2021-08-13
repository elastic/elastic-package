// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package system

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/aymerick/raymond"
	"github.com/pkg/errors"

	"github.com/elastic/go-ucfg"
	"github.com/elastic/go-ucfg/yaml"

	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/elastic-package/internal/testrunner"
	"github.com/elastic/elastic-package/internal/testrunner/runners/system/servicedeployer"
)

var systemTestConfigFilePattern = regexp.MustCompile(`^test-([a-z0-9_.-]+)-config.yml$`)

type testConfig struct {
	testrunner.SkippableConfig `config:",inline"`

	Input               string `config:"input"`
	Service             string `config:"service"`
	ServiceNotifySignal string `config:"service_notify_signal"` // Signal to send when the agent policy is applied.

	Vars       map[string]packages.VarValue `config:"vars"`
	DataStream struct {
		Vars map[string]packages.VarValue `config:"vars"`
	} `config:"data_stream"`

	// NumericKeywordFields holds a list of fields that have keyword
	// type but can be ingested as numeric type.
	NumericKeywordFields []string `config:"numeric_keyword_fields"`

	Path               string
	ServiceVariantName string
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

func newConfig(configFilePath string, ctxt servicedeployer.ServiceContext, serviceVariantName string) (*testConfig, error) {
	data, err := os.ReadFile(configFilePath)
	if err != nil && errors.Is(err, os.ErrNotExist) {
		return nil, errors.Wrapf(err, "unable to find system test configuration file: %s", configFilePath)
	}

	if err != nil {
		return nil, errors.Wrapf(err, "could not load system test configuration file: %s", configFilePath)
	}

	data, err = applyContext(data, ctxt)
	if err != nil {
		return nil, errors.Wrapf(err, "could not apply context to test configuration file: %s", configFilePath)
	}

	var c testConfig
	cfg, err := yaml.NewConfig(data, ucfg.PathSep("."))
	if err != nil {
		return nil, errors.Wrapf(err, "unable to load system test configuration file: %s", configFilePath)
	}
	if err := cfg.Unpack(&c); err != nil {
		return nil, errors.Wrapf(err, "unable to unpack system test configuration file: %s", configFilePath)
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
		return data, errors.Wrap(err, "parsing template body failed")
	}
	tmpl.RegisterHelpers(ctxt.Aliases())

	result, err := tmpl.Exec(ctxt)
	if err != nil {
		return data, errors.Wrap(err, "could not render data with context")
	}
	return []byte(result), nil
}
