package pipeline

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
)

const configTestSuffix = "-config.json"

type testConfig struct {
	Multiline *multiline             `json:"multiline"`
	Fields    map[string]interface{} `json:"fields"`
}

func readConfigForTestCase(testCasePath string) (testConfig, error) {
	var c testConfig

	configData, err := ioutil.ReadFile(filepath.Join(testCasePath, expectedTestConfigFile(testCasePath)))
	if err != nil && !os.IsNotExist(err) {
		return c, errors.Wrapf(err, "reading test config file failed (path: %s)", testCasePath)
	}

	if configData == nil {
		return c, nil
	}

	err = json.Unmarshal(configData, &c)
	if err != nil {
		return c, errors.Wrap(err, "unmarshalling test config failed")
	}
	return c, nil
}

func expectedTestConfigFile(testFile string) string {
	return fmt.Sprintf("%s%s", testFile, configTestSuffix)
}
