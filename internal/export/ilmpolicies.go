// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package export

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/elastic/elastic-package/internal/elasticsearch"
)

const ILMPoliciesExportDir = "ilm_policies"

func ILMPolicies(ctx context.Context, api *elasticsearch.API, output string, policies ...string) error {
	if len(policies) == 0 {
		return nil
	}

	policiesDir := filepath.Join(output, ILMPoliciesExportDir)
	err := os.MkdirAll(policiesDir, 0755)
	if err != nil {
		return fmt.Errorf("failed to create policies directory: %w", err)
	}

	for _, policy := range policies {
		err := exportILMPolicy(ctx, api, policiesDir, policy)
		if err != nil {
			return err
		}
	}
	return nil
}

func exportILMPolicy(ctx context.Context, api *elasticsearch.API, output string, policy string) error {
	resp, err := api.ILM.GetLifecycle(
		api.ILM.GetLifecycle.WithContext(ctx),
		api.ILM.GetLifecycle.WithPolicy(policy),
		api.ILM.GetLifecycle.WithPretty(),
	)
	if err != nil {
		return fmt.Errorf("failed to get policy %s: %w", policy, err)
	}
	defer resp.Body.Close()

	path := filepath.Join(output, policy+".json")

	w, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open file (%s) to export policy: %w", path, err)
	}
	defer w.Close()

	_, err = io.Copy(w, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to export to file: %w", err)
	}
	return nil
}
