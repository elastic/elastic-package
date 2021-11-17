// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package pipeline

import (
	"encoding/json"
	"strings"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
)

type Resource struct {
	Name    string
	Format  string
	Content []byte
	asJSON  []byte
}

func (p *Resource) FileName() string {
	pos := strings.LastIndexByte(p.Name, '-')
	if pos == -1 {
		pos = len(p.Name)
	}
	return p.Name[:pos] + "." + p.Format
}

func (p *Resource) JSON() ([]byte, error) {
	if len(p.asJSON) == 0 {
		if err := p.toJSON(); err != nil {
			return nil, err
		}
	}
	return p.asJSON, nil
}

func (p *Resource) toJSON() error {
	switch p.Format {
	case "json":
		p.asJSON = p.Content
	case "yaml", "yml":
		var node map[string]interface{}
		err := yaml.Unmarshal(p.Content, &node)
		if err != nil {
			return errors.Wrapf(err, "unmarshalling pipeline Content failed (pipeline: %s)", p.Name)
		}
		if p.asJSON, err = json.Marshal(&node); err != nil {
			return errors.Wrapf(err, "marshalling pipeline Content failed (pipeline: %s)", p.Name)
		}
	default:
		return errors.Errorf("unsupported pipeline Format '%s' for pipeline %s", p.Format, p.Name)
	}
	return nil
}
