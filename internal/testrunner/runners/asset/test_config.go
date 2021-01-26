package asset

import (
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"

	"github.com/elastic/go-ucfg"
	"github.com/elastic/go-ucfg/yaml"
	"github.com/pkg/errors"
)

type testConfig struct {
	// Skip allows this test to be skipped.
	Skip *struct {
		Reason string  `config:"reason"`
		Link   url.URL `config:"url"`
	} `config:"skip"`
}

func newConfig(assetTestFolderPath string) (*testConfig, error) {
	configFilePath := filepath.Join(assetTestFolderPath, "config.yml")

	// Test configuration file is optional for asset loading tests. If it
	// doesn't exist, we can return early.
	if _, err := os.Stat(configFilePath); os.IsNotExist(err) {
		return nil, nil
	}

	data, err := ioutil.ReadFile(configFilePath)
	if err != nil {
		return nil, errors.Wrapf(err, "could not load asset loading test configuration file: %s", configFilePath)
	}

	var c testConfig
	cfg, err := yaml.NewConfig(data, ucfg.PathSep("."))
	if err != nil {
		return nil, errors.Wrapf(err, "unable to load asset loading test configuration file: %s", configFilePath)
	}
	if err := cfg.Unpack(&c); err != nil {
		return nil, errors.Wrapf(err, "unable to unpack asset loading test configuration file: %s", configFilePath)
	}

	return &c, nil
}
