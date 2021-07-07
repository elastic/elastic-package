package servicedeployer

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
)

type variantsFile struct {
	Default  string `yaml:"default"`
	Variants map[string]environment
}

type environment map[string]string

type serviceVariant struct {
	name string
	env  []string // Environment variables in format of pairs: key=value
}

func (sv *serviceVariant) active() bool {
	return sv.name != ""
}

func (sv *serviceVariant) String() string {
	return fmt.Sprintf("ServiceVariant{name: %s, env: %s}", sv.name, strings.Join(sv.env, ","))
}

func useServiceVariant(devDeployPath, selected string) (serviceVariant, error) {
	variantsYmlPath := filepath.Join(devDeployPath, "variants.yml")
	_, err := os.Stat(variantsYmlPath)
	if errors.Is(err, os.ErrNotExist) {
		return serviceVariant{}, nil // no "variants.yml" present
	}
	if err != nil {
		return serviceVariant{}, errors.Wrap(err, "can't stat variants file")
	}

	content, err := ioutil.ReadFile(variantsYmlPath)
	if err != nil {
		return serviceVariant{}, errors.Wrap(err, "can't read variants file")
	}

	var f variantsFile
	err = yaml.Unmarshal(content, &f)
	if err != nil {
		return serviceVariant{}, errors.Wrap(err, "can't unmarshal variants file")
	}

	if selected == "" {
		selected = f.Default
	}

	if f.Default == "" {
		return serviceVariant{}, errors.New("default variant is undefined")
	}

	env, ok := f.Variants[selected]
	if !ok {
		return serviceVariant{}, fmt.Errorf(`variant "%s" is missing`, selected)
	}

	return serviceVariant{
		name: selected,
		env:  asEnvVarPairs(env),
	}, nil
}

func asEnvVarPairs(env environment) []string {
	var pairs []string
	for k, v := range env {
		pairs = append(pairs, fmt.Sprintf("%s=%s", k, v))
	}
	return pairs
}
