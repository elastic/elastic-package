// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package policy

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/pmezard/go-difflib/difflib"
	"gopkg.in/yaml.v3"

	"github.com/elastic/elastic-package/internal/common"
	"github.com/elastic/elastic-package/internal/kibana"
	"github.com/elastic/elastic-package/internal/logger"
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
	logger.Tracef("expected policy after cleaning:\n%s", want)
	logger.Tracef("found policy after cleaning:\n%s", got)

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
	onlyIfEmpty     bool
	ignoreValues    []any
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

	// Namespaces may not be present in older versions of the stack.
	{name: "namespaces", onlyIfEmpty: true, ignoreValues: []any{"default"}},

	// Values set by Fleet in input packages starting on 9.1.0.
	{name: "inputs", elementsEntries: []policyEntryFilter{
		{name: "streams", elementsEntries: []policyEntryFilter{
			{name: "data_stream.type"},
			{name: "data_stream.elasticsearch.dynamic_dataset"},
			{name: "data_stream.elasticsearch.dynamic_namespace"},
			{name: "data_stream.elasticsearch", onlyIfEmpty: true},
		}},
	}},
}

var uniqueOtelComponentIDReplace = policyEntryReplace{
	regexp:  regexp.MustCompile(`^(\s{2,})([^/]+)/([^:]+):(\s\{\}|\s*)$`),
	replace: "$1$2/componentid-%s:$4",
}

// otelComponentIDsRegexp is the regex to find otel components sections and their IDs to replace them with controlled values.
// It matches sections like:
//
//	 extensions:
//		  health_check/4391d954-1ffe-4014-a256-5eda78a71828: {}
//
//	 receivers:
//	     httpcheck/b0f518d6-4e2d-4c5d-bda7-f9808df537b7:
//	        collection_interval: 1m
//	        targets:
//	            - endpoints:
//	                - https://epr.elastic.co
//	              method: GET
//	   service:
//	      pipelines:
//	          logs:
//	              receivers/6b7f1379-dcb9-4ac7-b253-4df6d088b3ff:
//	                  - httpcheck/b0f518d6-4e2d-4c5d-bda7-f9808df537b7
//
// The regex captures the whole section, so it can be processed line by line to replace the IDs.
var otelComponentIDsRegexp = regexp.MustCompile(`(?m)^(?:extensions|receivers|processors|connectors|exporters|service):(?:\s\{\}\n|\n(?:\s{2,}.+\n)+)`)

// cleanPolicy prepares a policy YAML as returned by the download API to be compared with other
// policies. This preparation is based on removing contents that are generated, or replace them
// by controlled values.
func cleanPolicy(policy []byte, entriesToClean []policyEntryFilter) ([]byte, error) {
	// Replacement of the OTEL component IDs needs to be done before unmarshalling the YAML.
	// The OTEL IDs are keys in maps, and using the policyEntryFilter with memberReplace does
	// not ensure to keep the same ordering.
	policy = replaceOtelComponentIDs(policy)

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

// replaceOtelComponentIDs finds OTel Collector component IDs in the policy and replaces them with controlled values.
// It also replaces references to those IDs in service.extensions and service.pipelines.
func replaceOtelComponentIDs(policy []byte) []byte {
	replacementsDone := map[string]string{}

	policy = otelComponentIDsRegexp.ReplaceAllFunc(policy, func(match []byte) []byte {
		count := 0
		scanner := bufio.NewScanner(bytes.NewReader(match))
		var section strings.Builder
		for scanner.Scan() {
			line := scanner.Text()
			if uniqueOtelComponentIDReplace.regexp.MatchString(line) {
				originalOtelID, _, _ := strings.Cut(strings.TrimSpace(line), ":")

				replacement := fmt.Sprintf(uniqueOtelComponentIDReplace.replace, strconv.Itoa(count))
				count++
				line = uniqueOtelComponentIDReplace.regexp.ReplaceAllString(line, replacement)

				// store the otel ID replaced without the space indentation and the colon to be replaced later
				// (e.g. http_check/4391d954-1ffe-4014-a256-5eda78a71828 replaced by http_check/componentid-0)
				replacementsDone[originalOtelID], _, _ = strings.Cut(strings.TrimSpace(string(line)), ":")
			}
			section.WriteString(line + "\n")
		}

		return []byte(section.String())
	})

	// Replace references in arrays to the otel component IDs replaced before.
	// These references can be in:
	// service.extensions
	// service.pipelines.<signal>.(receivers|processors|exporters)
	for original, replacement := range replacementsDone {
		policy = bytes.ReplaceAll(policy, []byte(original), []byte(replacement))
	}
	return policy
}

func cleanPolicyMap(policyMap common.MapStr, entries []policyEntryFilter) (common.MapStr, error) {
	for _, entry := range entries {
		v, err := policyMap.GetValue(entry.name)
		if errors.Is(err, common.ErrKeyNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}

		switch {
		case len(entry.elementsEntries) > 0:
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
			m, ok := v.(common.MapStr)
			if !ok {
				return nil, fmt.Errorf("expected map, found %T", v)
			}
			regexp := entry.memberReplace.regexp
			replacement := entry.memberReplace.replace
			for k, e := range m {
				key := k
				if regexp.MatchString(k) {
					delete(m, k)
					key = regexp.ReplaceAllString(k, replacement)
					m[key] = e
				}
			}
		default:
			if entry.onlyIfEmpty && !isEmpty(v, entry.ignoreValues) {
				continue
			}
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

func isEmpty(v any, ignoreValues []any) bool {
	switch v := v.(type) {
	case nil:
		return true
	case []any:
		return len(filterIgnored(v, ignoreValues)) == 0
	case map[string]any:
		return len(v) == 0
	case common.MapStr:
		return len(v) == 0
	}

	return false
}

func filterIgnored(v []any, ignoredValues []any) []any {
	return slices.DeleteFunc(v, func(e any) bool {
		return slices.Contains(ignoredValues, e)
	})
}
