// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package policy

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/pmezard/go-difflib/difflib"
	"gopkg.in/yaml.v3"

	"github.com/elastic/elastic-package/internal/common"
	"github.com/elastic/elastic-package/internal/kibana"
)

func dumpExpectedAgentPolicy(ctx context.Context, kibanaClient *kibana.Client, testPath string, policyID string) error {
	policy, err := kibanaClient.DownloadPolicy(ctx, policyID)
	if err != nil {
		return fmt.Errorf("failed to download policy %q: %w", policyID, err)
	}

	d, err := cleanPolicy(policy, policyEntryFilters)
	if err != nil {
		return fmt.Errorf("failed to prepare policy to store: %w", err)
	}

	err = os.WriteFile(expectedPathFor(testPath), d, 0644)
	if err != nil {
		return fmt.Errorf("failed to write policy: %w", err)
	}

	return nil
}

func assertExpectedAgentPolicy(ctx context.Context, kibanaClient *kibana.Client, testPath string, policyID string) error {
	policy, err := kibanaClient.DownloadPolicy(ctx, policyID)
	if err != nil {
		return fmt.Errorf("failed to download policy %q: %w", policyID, err)
	}
	expectedPolicy, err := os.ReadFile(expectedPathFor(testPath))
	if err != nil {
		return fmt.Errorf("failed to read expected policy: %w", err)
	}

	diff, err := comparePolicies(expectedPolicy, policy)
	if err != nil {
		return fmt.Errorf("failed to compare policies: %w", err)
	}
	if len(diff) > 0 {
		return fmt.Errorf("unexpected content in policy: %s", diff)
	}

	return nil
}

func comparePolicies(expected, found []byte) (string, error) {
	want, err := cleanPolicy(expected, policyEntryFilters)
	if err != nil {
		return "", fmt.Errorf("failed to prepare expected policy: %w", err)
	}
	got, err := cleanPolicy(found, policyEntryFilters)
	if err != nil {
		return "", fmt.Errorf("failed to prepare found policy: %w", err)
	}
	if bytes.Equal(want, got) {
		return "", nil
	}

	var diff bytes.Buffer
	err = difflib.WriteUnifiedDiff(&diff, difflib.UnifiedDiff{
		A:        difflib.SplitLines(string(want)),
		B:        difflib.SplitLines(string(got)),
		FromFile: "want",
		ToFile:   "got",
		Context:  1,
	})
	if err != nil {
		return "", fmt.Errorf("failed to compare policies: %w", err)
	}
	return diff.String(), nil
}

func expectedPathFor(testPath string) string {
	ext := filepath.Ext(testPath)
	return strings.TrimSuffix(testPath, ext) + ".expected"
}

type policyEntryFilter struct {
	name            string
	elementsEntries []policyEntryFilter
	memberReplace   *policyEntryReplace
}

type policyEntryReplace struct {
	regexp  *regexp.Regexp
	replace string
}

// policyEntryFilter includes a list of filters to do to the policy. These filters
// are used to remove or control fields whose content is not relevant for the package
// test.
var policyEntryFilters = []policyEntryFilter{
	// IDs are not relevant.
	{name: "id"},
	{name: "inputs", elementsEntries: []policyEntryFilter{
		{name: "id"},
		{name: "package_policy_id"},
		{name: "streams", elementsEntries: []policyEntryFilter{
			{name: "id"},
		}},
	}},
	{name: "secret_references", elementsEntries: []policyEntryFilter{
		{name: "id"},
	}},

	// Avoid having to regenerate files every time the package version changes.
	{name: "inputs", elementsEntries: []policyEntryFilter{
		{name: "meta.package.version"},
	}},

	// Revision is not relevant, it is usually the same.
	{name: "revision"},
	{name: "inputs", elementsEntries: []policyEntryFilter{
		{name: "revision"},
	}},

	// Outputs, agent and fleet can depend on the deployment.
	{name: "agent"},
	{name: "fleet"},
	{name: "outputs"},

	// Signatures that change from installation to installation.
	{name: "agent.protection.uninstall_token_hash"},
	{name: "agent.protection.signing_key"},
	{name: "signed"},

	// We want to check permissions, but one is stored under a random UUID, replace it.
	{name: "output_permissions.default", memberReplace: &policyEntryReplace{
		// Match things that look like UUIDs.
		regexp:  regexp.MustCompile(`^[a-z0-9]{4,}(-[a-z0-9]{4,})+$`),
		replace: "uuid-for-permissions-on-related-indices",
	}},
}

// cleanPolicy prepares a policy YAML as returned by the download API to be compared with other
// policies. This preparation is based on removing contents that are generated, or replace them
// by controlled values.
func cleanPolicy(policy []byte, entriesToClean []policyEntryFilter) ([]byte, error) {
	var policyMap common.MapStr
	err := yaml.Unmarshal(policy, &policyMap)
	if err != nil {
		return nil, fmt.Errorf("failed to decode policy: %w", err)
	}

	policyMap, err = cleanPolicyMap(policyMap, entriesToClean)
	if err != nil {
		return nil, err
	}

	return yaml.Marshal(policyMap)
}

func cleanPolicyMap(policyMap common.MapStr, entries []policyEntryFilter) (common.MapStr, error) {
	for _, entry := range entries {
		switch {
		case len(entry.elementsEntries) > 0:
			v, err := policyMap.GetValue(entry.name)
			if errors.Is(err, common.ErrKeyNotFound) {
				continue
			}
			if err != nil {
				return nil, err
			}
			list, err := common.ToMapStrSlice(v)
			if err != nil {
				return nil, err
			}
			clean := make([]any, len(list))
			for i := range list {
				c, err := cleanPolicyMap(list[i], entry.elementsEntries)
				if err != nil {
					return nil, err
				}
				clean[i] = c
			}
			policyMap.Delete(entry.name)
			_, err = policyMap.Put(entry.name, clean)
			if err != nil {
				return nil, err
			}
		case entry.memberReplace != nil:
			v, err := policyMap.GetValue(entry.name)
			if errors.Is(err, common.ErrKeyNotFound) {
				continue
			}
			m, ok := v.(common.MapStr)
			if !ok {
				return nil, fmt.Errorf("expected map, found %T", v)
			}
			for k, e := range m {
				if entry.memberReplace.regexp.MatchString(k) {
					delete(m, k)
					m[entry.memberReplace.replace] = e
				}
			}
		default:
			err := policyMap.Delete(entry.name)
			if errors.Is(err, common.ErrKeyNotFound) {
				continue
			}
			if err != nil {
				return nil, err
			}
		}
	}

	return policyMap, nil
}
