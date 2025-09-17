// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package profile

import (
	"errors"
	"fmt"
	"os"

	"github.com/elastic/go-ucfg/yaml"
	"github.com/go-viper/mapstructure/v2"

	"github.com/elastic/elastic-package/internal/common"
)

const currentVersion = 1

type config struct {
	settings common.MapStr
}

func loadProfileConfig(path string) (config, error) {
	cfg, err := yaml.NewConfigWithFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return config{}, nil
	}
	if err != nil {
		return config{}, fmt.Errorf("can't load profile configuration (%s): %w", path, err)
	}

	settings := make(common.MapStr)
	err = cfg.Unpack(settings)
	if err != nil {
		return config{}, fmt.Errorf("can't unpack configuration: %w", err)
	}

	return config{settings: settings}, nil
}

func (c *config) get(name string) (string, bool) {
	raw, err := c.settings.GetValue(name)
	if err != nil {
		return "", false
	}
	switch v := raw.(type) {
	case string:
		return v, true
	default:
		return fmt.Sprintf("%v", v), true
	}
}

func (c *config) Decode(name string, out any) error {
	v, err := c.settings.GetValue(name)
	if err != nil {
		if errors.Is(err, common.ErrKeyNotFound) {
			return nil
		}
		return err
	}
	if err := mapstructure.Decode(v, out); err != nil {
		return err
	}

	return nil
}
