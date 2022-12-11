// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package system

import (
	"fmt"
	"os"
	"time"

	"github.com/pkg/errors"

	"github.com/elastic/go-ucfg"
	"github.com/elastic/go-ucfg/yaml"

	"github.com/elastic/elastic-package/internal/common"
	"github.com/elastic/elastic-package/internal/kibana"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/elastic-package/internal/testrunner/runners/system/servicedeployer"
)

type policyTemplateConfig struct {
	Name   string        `config:"name"`
	Inputs []inputConfig `config:"inputs"`
}

type inputConfig struct {
	Type    string        `config:"type"`
	Vars    common.MapStr `config:"vars"`
	Streams []struct {
		Dataset string        `config:"dataset"`
		Vars    common.MapStr `config:"vars"`
	} `config:"streams"`
}

type PackageConfig struct {
	PolicyTemplates []policyTemplateConfig `config:"policy_templates"`
	Path            string                 `config:",ignore"` // Path of config file.

	packageManifest *packages.PackageManifest      `config:",ignore"`
	dsManifests     []*packages.DataStreamManifest `config:",ignore"`
}

func NewPackageConfig(configFilePath string, packageRootPath string, ctxt servicedeployer.ServiceContext) (*PackageConfig, error) {
	data, err := os.ReadFile(configFilePath)
	if err != nil && errors.Is(err, os.ErrNotExist) {
		return nil, errors.Wrapf(err, "unable to find package configuration file: %s", configFilePath)
	}

	if err != nil {
		return nil, errors.Wrapf(err, "could not load package configuration file: %s", configFilePath)
	}

	data, err = applyContext(data, ctxt)
	if err != nil {
		return nil, errors.Wrapf(err, "could not apply context to package configuration file: %s", configFilePath)
	}

	var c PackageConfig
	cfg, err := yaml.NewConfig(data, ucfg.PathSep("."))
	if err != nil {
		return nil, errors.Wrapf(err, "unable to load packaage configuration file: %s", configFilePath)
	}
	if err := cfg.Unpack(&c); err != nil {
		return nil, errors.Wrapf(err, "unable to unpack package configuration file: %s", configFilePath)
	}
	// Save path
	c.Path = configFilePath

	c.packageManifest, err = packages.ReadPackageManifestFromPackageRoot(packageRootPath)
	if err != nil {
		return nil, errors.Wrapf(err, "reading package manifest failed (path: %s)", packageRootPath)
	}

	c.dsManifests, err = packages.ReadAllDataStreamManifests(packageRootPath)
	if err != nil {
		return nil, errors.Wrapf(err, "reading package manifest failed (path: %s)", packageRootPath)
	}

	return &c, nil
}

func (config *PackageConfig) CreatePackageDataStreams(
	kibanaPolicyID string,
) kibana.PackageDataStream {
	r := kibana.PackageDataStream{
		Name:      fmt.Sprintf("%s-%d", config.packageManifest.Name, time.Now().Unix()),
		Namespace: "default",
		PolicyID:  kibanaPolicyID,
		Enabled:   true,
	}

	r.Package.Name = config.packageManifest.Name
	r.Package.Title = config.packageManifest.Title
	r.Package.Version = config.packageManifest.Version

	for _, policyTemplateConfig := range config.PolicyTemplates {
		var policyTemplate packages.PolicyTemplate
		for _, pt := range config.packageManifest.PolicyTemplates {
			if pt.Name == policyTemplateConfig.Name {
				policyTemplate = pt
			}
		}

		if policyTemplate.Name == "" {
			logger.Warnf("invalid policy template \"%s\"\n", policyTemplateConfig.Name)
			continue
		}

		for _, inputCfg := range policyTemplateConfig.Inputs {
			inputType := inputCfg.Type

			input := kibana.Input{
				Type:           inputType,
				PolicyTemplate: policyTemplateConfig.Name,
				Enabled:        true,
				Vars:           setKibanaVariables(policyTemplate.FindInputByType(inputType).Vars, inputCfg.Vars),
			}

			for _, stream := range inputCfg.Streams {
				dataset := stream.Dataset
				dsManifest := config.findStreamForDataset(dataset)
				if dsManifest == nil {
					logger.Warn("could not find stream with dataset ", dataset)
					continue
				}

				stream := kibana.Stream{
					ID:      fmt.Sprintf("%s-%s.%s", inputType, config.packageManifest.Name, dsManifest.Name),
					Enabled: true,
					DataStream: kibana.DataStream{
						Type:    dsManifest.Type,
						Dataset: dsManifest.Dataset,
					},
					Vars: setKibanaVariables(dsManifest.Streams[0].Vars, stream.Vars),
				}
				input.Streams = append(input.Streams, stream)

				logger.Debug("added stream with dataset ", dataset)
			}

			r.Inputs = append(r.Inputs, input)
		}
	}

	return r
}

func (config *PackageConfig) findStreamForDataset(dataset string) *packages.DataStreamManifest {
	for _, dsm := range config.dsManifests {
		if dsm.Dataset == dataset {
			return dsm
		}
	}
	return nil
}
