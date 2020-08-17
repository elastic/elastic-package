package formatter

import (
	"gopkg.in/yaml.v3"

	"github.com/pkg/errors"
)

func yamlFormatter(content []byte) ([]byte, bool, error) {
	// yaml.Unmarshal() requires `yaml.Node` to be passed instead of generic `interface{}`.
	// Otherwise it can detect any comments and fields are considered as normal map.
	var node yaml.Node
	err := yaml.Unmarshal(content, &node)
	if err != nil {
		return nil, false, errors.Wrap(err, "unmarshalling YAML file failed")
	}

	formatted, err := yaml.Marshal(&node)
	if err != nil {
		return nil, false, errors.Wrap(err, "marshalling YAML node failed")
	}
	return formatted, string(content) == string(formatted), nil
}
