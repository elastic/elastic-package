package system

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/aymerick/raymond"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"

	"github.com/elastic/elastic-package/internal/common"
	"github.com/elastic/elastic-package/internal/packages"
)

const configFileName = "config.yml"

type config struct {
	Vars    map[string]packages.VarValue `yaml:"vars"`
	Dataset struct {
		Vars map[string]packages.VarValue `yaml:"vars"`
	} `yaml:"dataset"`
}

func newConfig(systemTestFolderPath string, ctxt common.MapStr) (*config, error) {
	configFilePath := filepath.Join(systemTestFolderPath, configFileName)
	data, err := ioutil.ReadFile(configFilePath)
	if err != nil && os.IsNotExist(err) {
		return nil, errors.Wrapf(err, "unable to find system test configuration file: %s", configFilePath)
	}

	if err != nil {
		return nil, errors.Wrapf(err, "could not load system test configuration file: %s", configFilePath)
	}

	data, err = applyContext(ctxt, data)
	if err != nil {
		return nil, errors.Wrapf(err, "could not apply context to test configuration file: %s", configFilePath)
	}

	var c config
	if err := yaml.Unmarshal(data, &c); err != nil {
		return nil, errors.Wrapf(err, "unable to parse system test configuration file: %s", configFilePath)
	}

	return &c, nil
}

func applyContext(ctxt common.MapStr, data []byte) ([]byte, error) {
	tpl := string(data)
	result, err := raymond.Render(tpl, ctxt)
	if err != nil {
		return data, errors.Wrap(err, "could not render data with context")
	}

	return []byte(result), nil
}
