// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package profile

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/elastic/go-ucfg/yaml"

	"github.com/elastic/elastic-package/internal/common"
)

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

func (c *config) Unmarshal(name string, out any) error {
	v, err := c.settings.GetValue(name)
	if err != nil {
		if errors.Is(err, common.ErrKeyNotFound) {
			return nil
		}
		return err
	}

	data, err := json.Marshal(v)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(data, out); err != nil {
		return err
	}

	return nil
}
