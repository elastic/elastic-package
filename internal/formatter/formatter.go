package formatter

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
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

	switch ext {
	case ".json":
	case ".yml":
	default:
		return nil
	}

	content, err := ioutil.ReadFile(path)
	if err != nil {
		return errors.Wrap(err, "reading file content failed")
	}

	var formatted []byte
	switch ext {
	case ".json":
		var rawMessage json.RawMessage
		err = json.Unmarshal(content, &rawMessage)
		if err != nil {
			return errors.Wrap(err, "unmarshalling JSON file failed")
		}

		formatted, err = json.MarshalIndent(&rawMessage, "", "    ")
		if err != nil {
			return errors.Wrap(err, "marshalling JSON raw message failed")
		}
	case ".yml":
		var node yaml.Node
		err = yaml.Unmarshal(content, &node)
		if err != nil {
			return errors.Wrap(err, "unmarshalling YAML file failed")
		}

		formatted, err = yaml.Marshal(&node)
		if err != nil {
			return errors.Wrap(err, "marshalling JSON raw message failed")
		}
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
