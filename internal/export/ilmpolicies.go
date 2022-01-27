// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package export

import (
	"context"
	"fmt"
	"io/ioutil"

	"github.com/elastic/elastic-package/internal/elasticsearch"
)

type ILMPolicy struct {
	name string
	raw  []byte
}

func (p ILMPolicy) Name() string {
	return p.name
}

func (p ILMPolicy) JSON() []byte {
	return p.raw
}

func getILMPolicies(ctx context.Context, api *elasticsearch.API, policies ...string) ([]ILMPolicy, error) {
	if len(policies) == 0 {
		return nil, nil
	}

	var ilmPolicies []ILMPolicy
	for _, policy := range policies {
		ilmPolicy, err := getILMPolicyByName(ctx, api, policy)
		if err != nil {
			return nil, err
		}
		ilmPolicies = append(ilmPolicies, ilmPolicy)
	}
	return ilmPolicies, nil
}

func getILMPolicyByName(ctx context.Context, api *elasticsearch.API, policy string) (ILMPolicy, error) {
	resp, err := api.ILM.GetLifecycle(
		api.ILM.GetLifecycle.WithContext(ctx),
		api.ILM.GetLifecycle.WithPolicy(policy),
		api.ILM.GetLifecycle.WithPretty(),
	)
	if err != nil {
		return ILMPolicy{}, fmt.Errorf("failed to get policy %s: %w", policy, err)
	}
	defer resp.Body.Close()

	// TODO: Handle the case of a response with multiple policies (no policy, or with wildcard).
	// TODO: Get the actual policy from the returned object.
	d, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return ILMPolicy{}, fmt.Errorf("failed to read response body: %w", err)
	}

	return ILMPolicy{name: policy, raw: d}, nil
}
