// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package dump

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"

	"github.com/elastic/elastic-package/internal/elasticsearch"
)

// ILMPolicy contains the information needed to export an ILM policy.
type ILMPolicy struct {
	name string
	raw  json.RawMessage
}

// Name returns the name of the ILM policy.
func (p ILMPolicy) Name() string {
	return p.name
}

// JSON returns the JSON representation of the ILM policy.
func (p ILMPolicy) JSON() []byte {
	return p.raw
}

func getILMPolicies(ctx context.Context, api *elasticsearch.API, policies ...string) ([]ILMPolicy, error) {
	if len(policies) == 0 {
		return nil, nil
	}

	var ilmPolicies []ILMPolicy
	for _, policy := range policies {
		resultPolicies, err := getILMPolicyByName(ctx, api, policy)
		if err != nil {
			return nil, err
		}
		ilmPolicies = append(ilmPolicies, resultPolicies...)
	}
	return ilmPolicies, nil
}

type getILMLifecycleResponse map[string]json.RawMessage

func getILMPolicyByName(ctx context.Context, api *elasticsearch.API, policy string) ([]ILMPolicy, error) {
	resp, err := api.ILM.GetLifecycle(
		api.ILM.GetLifecycle.WithContext(ctx),
		api.ILM.GetLifecycle.WithPolicy(policy),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get policy %s: %w", policy, err)
	}
	defer resp.Body.Close()

	d, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var policiesResponse getILMLifecycleResponse
	err = json.Unmarshal(d, &policiesResponse)
	if err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	var policies []ILMPolicy
	for name, raw := range policiesResponse {
		policies = append(policies, ILMPolicy{name: name, raw: raw})
	}

	return policies, nil
}
