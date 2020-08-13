package formatter

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
)

// Format method formats files inside of the integration directory.
func Format(packageRoot string, failFast bool) error {
	err := filepath.Walk(packageRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}
		err = formatFile(path, failFast)
		if err != nil {
			return errors.Wrapf(err, "formatting file failed (path: %s)", path)
		}

		return nil
	})
	if err != nil {
		return errors.Wrap(err, "walking through the integration files failed")
	}
	return nil
}

func formatFile(path string, failFast bool) error {
	file := filepath.Base(path)
	ext := filepath.Ext(file)

	var c interface{}
	var unmarshal func([]byte, interface{}) error
	var marshal func(interface{}, string, string) ([]byte, error)
	switch ext {
	case ".json":
		c = json.RawMessage{}
		unmarshal = json.Unmarshal
		marshal = json.MarshalIndent
	case ".yml":
		c = yaml.MapSlice{}
		unmarshal = yaml.Unmarshal
		marshal = func(v interface{}, prefix string, indent string) ([]byte, error) {
			return yaml.Marshal(v)
		}
	default:
		return nil // file won't be formatted
	}

	content, err := ioutil.ReadFile(path)
	if err != nil {
		return errors.Wrap(err, "reading file content failed ")
	}

	err = unmarshal(content, &c)
	if err != nil {
		return errors.Wrap(err, "unmarshalling file failed")
	}

	formatted, err := marshal(c, " ", " ")
	if err != nil {
		return errors.Wrap(err, "marshalling file failed ")
	}

	if string(content) == string(formatted) {
		return nil // file is already in good shape
	}

	if failFast {
		return fmt.Errorf("file is not formatted (path: %s)", path)
	}

	err = ioutil.WriteFile(path, formatted, 0755)
	if err != nil {
		return errors.Wrapf(err, "rewriting file failed (path: %s)", path)
	}
	return nil
}
