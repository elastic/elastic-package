// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package system

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/aymerick/raymond"
	"github.com/pkg/errors"

	"github.com/elastic/go-ucfg"
	"github.com/elastic/go-ucfg/yaml"

	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/elastic-package/internal/testrunner/runners/system/servicedeployer"
)

const configFileName = "config.yml"

type testConfig struct {
	Vars       map[string]packages.VarValue `config:"vars"`
	DataStream struct {
		Vars map[string]packages.VarValue `config:"vars"`
	} `config:"data_stream"`
}

func newConfig(systemTestFolderPath string, ctxt servicedeployer.ServiceContext) (*testConfig, error) {
	configFilePath := filepath.Join(systemTestFolderPath, configFileName)
	data, err := ioutil.ReadFile(configFilePath)
	if err != nil && os.IsNotExist(err) {
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
	return &c, nil
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
