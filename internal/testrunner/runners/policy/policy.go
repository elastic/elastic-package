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
	"github.com/elastic/elastic-package/internal/testrunner"
)

func dumpExpectedAgentPolicy(ctx context.Context, options testrunner.TestOptions, testPath string, policyID string) error {
	policy, err := options.KibanaClient.DownloadPolicy(ctx, policyID)
	if err != nil {
		return fmt.Errorf("failed to download policy %q: %w", policyID, err)
	}

	err = os.WriteFile(expectedPathFor(testPath), policy, 0644)
	if err != nil {
		return fmt.Errorf("failed to write policy: %w", err)
	}

	return nil
}

func assertExpectedAgentPolicy(ctx context.Context, options testrunner.TestOptions, testPath string, policyID string) error {
	policy, err := options.KibanaClient.DownloadPolicy(ctx, policyID)
	if err != nil {
		return fmt.Errorf("failed to download policy %q: %w", policyID, err)
	}
	expectedPolicy, err := os.ReadFile(expectedPathFor(testPath))
	if err != nil {
		return fmt.Errorf("failed to read expected policy: %w", err)
	}

	return comparePolicies(expectedPolicy, policy)
}

func comparePolicies(expected, found []byte) error {
	want, err := cleanPolicy(expected)
	if err != nil {
		return fmt.Errorf("failed to prepare expected policy: %w", err)
	}
	got, err := cleanPolicy(found)
	if err != nil {
		return fmt.Errorf("failed to prepare found policy: %w", err)
	}

	if bytes.Equal(want, got) {
		return nil
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
		return fmt.Errorf("failed to compare policies: %w", err)
	}
	return fmt.Errorf("unexpected content in policy: %s", diff.String())
}

func expectedPathFor(testPath string) string {
	ext := filepath.Ext(testPath)
	return strings.TrimSuffix(testPath, ext) + ".expected"
}

type policyEntry struct {
	name            string
	elementsEntries []policyEntry
	memberReplace   *policyEntryReplace
}

type policyEntryReplace struct {
	regexp  *regexp.Regexp
	replace string
}

var policyEntriesToIgnore = []policyEntry{
	{name: "id"},
	{name: "outputs.ssl.ca_trusted_fingerprint"},
	{name: "agent.protection.uninstall_token_hash"},
	{name: "agent.protection.signing_key"},
	{name: "signed"},
	{name: "inputs", elementsEntries: []policyEntry{
		{name: "id"},
		{name: "package_policy_id"},
		{name: "streams", elementsEntries: []policyEntry{
			{name: "id"},
		}},
	}},
	{name: "output_permissions.default", memberReplace: &policyEntryReplace{
		// Match things that look like UUIDs.
		regexp:  regexp.MustCompile(`^[a-z0-9]{4,}(-[a-z0-9]{4,})+$`),
		replace: "aaaaaaaa-bbbb-cccc-dddd-eeeeeeee",
	}},
}

// cleanPolicy prepares a policy YAML as returned by the download API to be compared with other
// policies. This preparation is based on removing contents that are generated, or replace them
// by controlled values.
func cleanPolicy(policy []byte) ([]byte, error) {
	var policyMap common.MapStr
	err := yaml.Unmarshal(policy, &policyMap)
	if err != nil {
		return nil, fmt.Errorf("failed to decode policy: %w", err)
	}

	policyMap, err = cleanPolicyMap(policyMap, policyEntriesToIgnore)
	if err != nil {
		return nil, err
	}

	return yaml.Marshal(policyMap)
}

func cleanPolicyMap(policyMap common.MapStr, entries []policyEntry) (common.MapStr, error) {
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
