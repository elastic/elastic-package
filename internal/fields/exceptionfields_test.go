// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package fields

import (
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/elastic-package/internal/common"
)

func TestFindExceptionElements(t *testing.T) {
	for _, test := range []struct {
		title       string
		doc         common.MapStr
		definition  []FieldDefinition
		expected    []string
		specVersion semver.Version
	}{
		{
			title: "exception for type array",
			doc: common.MapStr{
				"remote_ip_list": []any{
					"::1",
					"127.0.0.1",
				},
			},
			definition: []FieldDefinition{
				{
					Name: "remote_ip_list",
					Type: "array",
				},
			},
			expected:    []string{"remote_ip_list"},
			specVersion: *semver.MustParse("1.5.0"),
		},
		{
			title: "exception for type nested",
			doc: common.MapStr{
				"ip_port_info": []any{
					map[string]any{
						"ip_protocol": []any{
							"ALL",
						},
						"port_range": []any{
							"80",
							"8080",
						},
					},
					map[string]any{
						"ip_protocol": []any{
							"TCP",
						},
						"port_range": []any{
							"8888",
						},
					},
				},
			},
			definition: []FieldDefinition{
				{
					Name: "ip_port_info",
					Type: "nested",
				},
			},
			expected:    []string{"ip_port_info"},
			specVersion: *semver.MustParse("1.5.0"),
		},
		{
			title: "exception for type group",
			doc: common.MapStr{
				"ip_port_info": []any{
					map[string]any{
						"ip_protocol": []any{
							"ALL",
						},
						"port_range": []any{
							"80",
							"8080",
						},
					},
					map[string]any{
						"ip_protocol": []any{
							"TCP",
						},
						"port_range": []any{
							"8888",
						},
					},
				},
				"answers": []any{
					map[string]any{
						"data":       "dns1.com",
						"preference": 1,
						"name":       "elastic.co",
						"type":       "MX",
						"class":      "IN",
						"ttl":        "276",
					},
					map[string]any{
						"data":       "dns2.com",
						"preference": 10,
						"name":       "elastic.co",
						"type":       "MX",
						"class":      "IN",
						"ttl":        "276",
					},
				},
			},
			definition: []FieldDefinition{
				{
					Name: "answers",
					Type: "group",
				},
			},
			expected:    []string{"answers"},
			specVersion: *semver.MustParse("1.5.0"),
		},
		{
			title: "no exceptions",
			doc: common.MapStr{
				"remote_ip_list": []any{
					"::1",
					"127.0.0.1",
				},
				"answers": []any{
					map[string]any{
						"data":       "dns1.com",
						"preference": 1,
						"name":       "elastic.co",
						"type":       "MX",
						"class":      "IN",
						"ttl":        "276",
					},
					map[string]any{
						"data":       "dns2.com",
						"preference": 10,
						"name":       "elastic.co",
						"type":       "MX",
						"class":      "IN",
						"ttl":        "276",
					},
				},
			},
			definition: []FieldDefinition{
				{
					Name: "answers",
					Type: "group",
				},
				{
					Name: "ip_port_info",
					Type: "nested",
				},
				{
					Name: "ip_port_info",
					Type: "nested",
				},
			},
			expected:    []string{},
			specVersion: *semver.MustParse("3.3.0"),
		},
	} {

		t.Run(test.title, func(t *testing.T) {
			v := Validator{
				Schema:                       test.definition,
				disabledDependencyManagement: true,
				specVersion:                  test.specVersion,
			}

			fields := v.ListExceptionFields(test.doc)

			require.Len(t, fields, len(test.expected))
			assert.Equal(t, test.expected, fields)
		})
	}
}
