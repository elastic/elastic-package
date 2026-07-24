// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package policy

import (
	"bytes"
	"context"
	"encoding/json"
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
	logger.Tracef("expected policy before cleaning:\n%s", string(expected))
	logger.Tracef("found policy before cleaning:\n%s", string(found))
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
	name               string
	elementsEntries    []policyEntryFilter
	mapValues          []policyEntryFilter
	memberReplace      *policyEntryReplace
	stringValueReplace *policyEntryReplace
	deletePattern      *regexp.Regexp
	onlyIfEmpty        bool
	ignoreValues       []any
}

type policyEntryReplace struct {
	regexp  *regexp.Regexp
	replace string
}

// beatsauthPattern matches OTel component IDs for the beatsauth extension injected by Fleet.
var beatsauthPattern = regexp.MustCompile(`^beatsauth/`)

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
		{name: "name", stringValueReplace: &policyEntryReplace{
			regexp:  regexp.MustCompile(`^(.+)-[0-9]+$`),
			replace: "$1",
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
	{name: "exporters", mapValues: []policyEntryFilter{
		{name: "endpoints", memberReplace: &policyEntryReplace{
			regexp:  regexp.MustCompile(`^https?://.*$`),
			replace: "https://elasticsearch:9200",
		}},
		// auth is injected by Fleet since 9.4.0 and may appear in any exporter, not just
		// elasticsearch. Removed for backwards compatibility with older stacks.
		{name: "auth"},
	}},

	// Fields injected by Fleet into OTel policies since 9.4.0 (beatsauth extension).
	{name: "extensions", deletePattern: beatsauthPattern},
	{name: "extensions", onlyIfEmpty: true},
	{name: "service.extensions", deletePattern: beatsauthPattern},
	{name: "service.extensions", onlyIfEmpty: true},

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

	// Fields present since 9.3.0.
	{name: "inputs", elementsEntries: []policyEntryFilter{
		{name: "meta.package.policy_template"},
		{name: "meta.package.release"},
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

	data, err := yaml.Marshal(policyMap)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal policy: %w", err)
	}

	return normalizePolicyToCanonical(data)
}

// otelVariableKeySections are top-level keys whose map entries use variable IDs (type/id).
var otelVariableKeySections = []string{"extensions", "receivers", "processors", "connectors", "exporters"}

// isOTelVariableKey returns true for keys that are OTel component IDs (e.g. "zipkin/componentid-0", "elasticsearch/default").
func isOTelVariableKey(key string) bool {
	return strings.Contains(key, "/")
}

// ottlConditionalDataStreamAttr matches " where attributes["data_stream.<field>"] == nil"
// suffixes appended by Fleet since kibana#274993. Stripping them makes policy comparison
// stable across Fleet versions that do and don't add the guard.
var ottlConditionalDataStreamAttr = regexp.MustCompile(` where attributes\["data_stream\.\w+"\] == nil$`)

// preNormalizePolicy rewrites the decoded policy tree before component-ID normalization
// to absorb Fleet version differences that would otherwise cause spurious policy test failures.
func preNormalizePolicy(root map[string]any) {
	// Rename the bare "forward" connector key to "forward/_bare" so it is treated as a
	// variable key and participates in normalization. Fleet added an output-ID suffix in
	// 9.4.3 (kibana#270487) producing "forward/<outputId>" keys, which already contain "/"
	// and are normalized automatically. Only the bare case needs renaming here; distinct
	// "forward/<outputId>" keys are left intact so policies with multiple outputs retain
	// separate forward connectors.
	if connectors, ok := toMap(root["connectors"]); ok {
		if v, hasBare := connectors["forward"]; hasBare {
			if _, taken := connectors["forward/_bare"]; !taken {
				delete(connectors, "forward")
				connectors["forward/_bare"] = v
			}
		}
	}

	// Rename bare pipeline keys (e.g. "logs", "metrics") to "<signal>/_bare" so
	// they participate in normalization the same way suffixed keys do. Fleet
	// started suffixing these with the output ID in 9.4.3 (kibana#270487).
	if service, ok := toMap(root["service"]); ok {
		if pipelines, ok := toMap(service["pipelines"]); ok {
			for k, v := range pipelines {
				if !strings.Contains(k, "/") {
					target := k + "/_bare"
					if _, taken := pipelines[target]; !taken {
						delete(pipelines, k)
						pipelines[target] = v
					}
				}
			}
		}
	}

	// Rename bare extension map keys (those without "/") to "<name>/_bare" so they
	// participate in component-ID normalization. Fleet started suffixing these with a
	// component ID in 9.5.0, so older expected files may still use bare keys.
	// String references to these extensions at known positions are resolved later by
	// resolveExtensionRefs, after buildSectionMapping has established the canonical IDs.
	if extensions, ok := toMap(root["extensions"]); ok {
		for k, v := range extensions {
			if !strings.Contains(k, "/") {
				target := k + "/_bare"
				if _, taken := extensions[target]; !taken {
					delete(extensions, k)
					extensions[target] = v
				}
			}
		}
	}

	// Walk string elements in arrays to:
	//   - replace bare "forward" connector refs with "forward/_bare" (pipeline arrays)
	//   - strip conditional "where ... == nil" OTTL suffixes from set() statements
	preNormalizeNode(root)
}

// preNormalizeNode recursively walks the tree. String elements inside slices are rewritten;
// map keys are left to the later normalization pass. Extension string references are NOT
// touched here — they are resolved at known structural positions by resolveExtensionRefs.
func preNormalizeNode(node any) {
	switch n := node.(type) {
	case map[string]any:
		for _, v := range n {
			preNormalizeNode(v)
		}
	case []any:
		for i, elem := range n {
			if s, ok := elem.(string); ok {
				s = ottlConditionalDataStreamAttr.ReplaceAllString(s, "")
				if s == "forward" {
					s = "forward/_bare"
				}
				n[i] = s
			} else {
				preNormalizeNode(elem)
			}
		}
	}
}

// normalizePolicyToCanonical rewrites OTel component IDs to deterministic type/componentid-N
// and updates all references. It works on the decoded tree and sorts variable keys by
// canonical value so that equivalent policies with different map key order normalize to
// the same output.
func normalizePolicyToCanonical(policy []byte) ([]byte, error) {
	var root map[string]any
	if err := yaml.Unmarshal(policy, &root); err != nil {
		return nil, fmt.Errorf("failed to decode policy: %w", err)
	}

	preNormalizePolicy(root)

	// Build mapping oldKey -> newKey (e.g. "elasticsearch/default" -> "elasticsearch/componentid-0")
	// by processing each variable-key section with deterministic (value-based) key order.
	idMapping := make(map[string]string)

	for _, section := range otelVariableKeySections {
		v, ok := root[section]
		if !ok {
			continue
		}
		m, ok := toMap(v)
		if !ok {
			continue
		}
		buildSectionMapping(m, idMapping)
	}

	// service.pipelines: keys are pipeline names (variable when they contain "/")
	if service, ok := toMap(root["service"]); ok {
		if pipelines, ok := toMap(service["pipelines"]); ok {
			buildSectionMapping(pipelines, idMapping)
		}
	}

	// Resolve bare extension type-name references at known OTel structural positions:
	// service.extensions list items, *.auth.authenticator values, and *.middlewares[].id
	// values. This handles two states: fully-bare expected files (where extension map keys
	// were renamed to _bare above) and mixed-state files (where the map key already has a
	// suffix but references still use the bare type name).
	resolveExtensionRefs(root, idMapping)

	// Apply mapping: replace keys in variable-key maps and replace string references in the whole tree.
	applyNormalization(root, idMapping)

	return yaml.Marshal(root)
}

// toMap returns v as map[string]any.
func toMap(v any) (map[string]any, bool) {
	if v == nil {
		return nil, false
	}
	m, ok := v.(map[string]any)
	return m, ok
}

// canonicalValueKey returns a byte slice that can be used to sort values deterministically.
func canonicalValueKey(v any) ([]byte, error) {
	return json.Marshal(v)
}

// buildSectionMapping adds oldKey -> newKey entries for variable keys in m, sorted by canonical value.
func buildSectionMapping(m map[string]any, idMapping map[string]string) {
	var variableKeys []string
	for k := range m {
		if isOTelVariableKey(k) {
			variableKeys = append(variableKeys, k)
		}
	}
	if len(variableKeys) == 0 {
		return
	}
	slices.SortFunc(variableKeys, func(a, b string) int {
		// First compare the canonical value of the keys (content of the map entry)
		va, _ := canonicalValueKey(m[a])
		vb, _ := canonicalValueKey(m[b])
		if c := bytes.Compare(va, vb); c != 0 {
			return c
		}
		// If the canonical values are the same, compare the keys lexicographically
		return strings.Compare(a, b)
	})
	for i, oldKey := range variableKeys {
		typ, _, _ := strings.Cut(oldKey, "/")
		if typ == "" {
			typ = "component"
		}
		newKey := typ + "/componentid-" + strconv.Itoa(i)
		idMapping[oldKey] = newKey
	}
}

// applyNormalization replaces keys in variable-key maps and replaces string values that are component refs.
func applyNormalization(node any, idMapping map[string]string) {
	switch n := node.(type) {
	case map[string]any:
		// Detect if this is a variable-key section by checking keys.
		hasVariableKeys := false
		for k := range n {
			if isOTelVariableKey(k) {
				hasVariableKeys = true
				break
			}
		}
		if hasVariableKeys {
			// Recurse into values first, then replace keys.
			for _, v := range n {
				applyNormalization(v, idMapping)
			}
			newMap := make(map[string]any, len(n))
			for k, v := range n {
				newKey := k
				if nk, ok := idMapping[k]; ok {
					newKey = nk
				}
				newMap[newKey] = v
			}
			// delete the original map entries
			for k := range n {
				delete(n, k)
			}
			// add the new map entried with the new keys
			for k, v := range newMap {
				n[k] = v
			}
			return
		}
		for k, v := range n {
			n[k] = replaceOrRecurse(v, idMapping)
		}
	case []any:
		for i, elem := range n {
			n[i] = replaceOrRecurse(elem, idMapping)
		}
	default:
		// strings, numbers, etc. — no change
	}
}

// replaceOrRecurse returns v's canonical replacement if v is a string found in idMapping;
// otherwise it recurses into v (for maps/slices) and returns v unchanged.
func replaceOrRecurse(v any, idMapping map[string]string) any {
	if s, ok := v.(string); ok {
		if newRef, ok := idMapping[s]; ok {
			return newRef
		}
		return v
	}
	applyNormalization(v, idMapping)
	return v
}

// resolveExtensionRefs rewrites bare extension type-name strings at the three known OTel
// reference positions — service.extensions list items, auth.authenticator scalar values, and
// middlewares[].id values inside list elements — mapping them to their canonical component IDs.
//
// It covers two states of expected files:
//   - Mixed state: extension map key already suffixed (e.g. basicauth/componentid-0) but
//     references still use the bare type name (e.g. authenticator: basicauth).
//   - Fully-bare state: extension map key was renamed to _bare by preNormalizePolicy; the
//     reference is still the bare type name since preNormalizeNode no longer renames it.
//
// If every extension key and every reference already use a suffixed form (fully-suffixed state),
// typeToCanonical is built from those suffixed keys but contains only bare type names as keys
// (e.g. "basicauth"). The walker then looks for bare type names at reference positions, finds
// none (all references already contain "/"), and makes no changes — the function is a no-op.
//
// If multiple extensions share the same type prefix the type is excluded from the mapping to
// avoid non-deterministic resolution; the expected file must use full canonical IDs in that case.
func resolveExtensionRefs(root map[string]any, idMapping map[string]string) {
	typeToCanonical := buildExtensionTypeMapping(root, idMapping)
	if len(typeToCanonical) == 0 {
		return
	}

	// Resolve service.extensions list items (direct extension ID strings).
	if svc, ok := toMap(root["service"]); ok {
		if exts, ok := svc["extensions"].([]any); ok {
			for i, v := range exts {
				if s, ok := v.(string); ok {
					if canonical, found := typeToCanonical[s]; found {
						exts[i] = canonical
					}
				}
			}
		}
	}

	// Resolve auth.authenticator and middlewares[].id throughout the rest of the tree.
	resolveExtensionRefNode(root, typeToCanonical)
}

// buildExtensionTypeMapping returns a map from bare extension type name (e.g. "basicauth") to
// its canonical component ID (e.g. "basicauth/componentid-0"), derived from the extension keys
// and their idMapping entries. Types with more than one extension are excluded to prevent
// non-deterministic resolution.
func buildExtensionTypeMapping(root map[string]any, idMapping map[string]string) map[string]string {
	extMap, ok := toMap(root["extensions"])
	if !ok {
		return nil
	}

	typeCounts := make(map[string]int)
	for k := range extMap {
		typ, _, hasSlash := strings.Cut(k, "/")
		if hasSlash && typ != "" {
			typeCounts[typ]++
		}
	}

	typeToCanonical := make(map[string]string)
	for k := range extMap {
		typ, _, hasSlash := strings.Cut(k, "/")
		if !hasSlash || typ == "" || typeCounts[typ] > 1 {
			continue
		}
		if canonical, found := idMapping[k]; found {
			typeToCanonical[typ] = canonical
		}
	}
	return typeToCanonical
}

// resolveExtensionRefNode walks the tree and replaces bare extension type-name strings at the
// two known sub-tree reference positions: the authenticator key (scalar string value) and the
// id key inside middlewares list elements.
func resolveExtensionRefNode(node any, typeToCanonical map[string]string) {
	switch n := node.(type) {
	case map[string]any:
		for k, v := range n {
			switch k {
			case "authenticator":
				if s, ok := v.(string); ok {
					if canonical, found := typeToCanonical[s]; found {
						n[k] = canonical
					}
				}
			case "middlewares":
				if list, ok := v.([]any); ok {
					for _, elem := range list {
						if m, ok := toMap(elem); ok {
							if idVal, ok := m["id"].(string); ok {
								if canonical, found := typeToCanonical[idVal]; found {
									m["id"] = canonical
								}
							}
						}
					}
				}
			default:
				resolveExtensionRefNode(v, typeToCanonical)
			}
		}
	case []any:
		for _, elem := range n {
			resolveExtensionRefNode(elem, typeToCanonical)
		}
	}
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
			if err := applyElementsEntriesCleaning(policyMap, entry.name, v, entry.elementsEntries); err != nil {
				return nil, err
			}
		case len(entry.mapValues) > 0:
			if err := applyMapValuesCleaning(v, entry.mapValues); err != nil {
				return nil, err
			}
		case entry.memberReplace != nil:
			if err := applyMemberReplace(policyMap, entry.name, v, entry.memberReplace); err != nil {
				return nil, err
			}
		case entry.stringValueReplace != nil:
			if err := applyStringValueReplace(policyMap, entry.name, v, entry.stringValueReplace); err != nil {
				return nil, err
			}
		case entry.deletePattern != nil:
			if err := applyDeletePattern(policyMap, entry.name, v, entry.deletePattern); err != nil {
				return nil, err
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

// replaceMapStrValue replaces the value stored at key in m by deleting it and putting the new value.
// Delete before Put is required because MapStr.Put does not overwrite slice/map values in place.
func replaceMapStrValue(m common.MapStr, key string, value any) error {
	m.Delete(key)
	_, err := m.Put(key, value)
	return err
}

// cleanPolicyMapStrSlice applies cleanPolicyMap to every element of list and returns a []any
// suitable for storing back into a MapStr.
func cleanPolicyMapStrSlice(list []common.MapStr, filters []policyEntryFilter) ([]any, error) {
	clean := make([]any, len(list))
	for i := range list {
		c, err := cleanPolicyMap(list[i], filters)
		if err != nil {
			return nil, err
		}
		clean[i] = c
	}
	return clean, nil
}

// cleanNestedPolicyValue applies filters to v, which must be either a single MapStr or a slice
// of MapStr values. It returns the cleaned value (MapStr or []any) to store back.
func cleanNestedPolicyValue(v any, filters []policyEntryFilter) (any, error) {
	if vMap, err := common.ToMapStr(v); err == nil {
		return cleanPolicyMap(vMap, filters)
	}
	if list, err := common.ToMapStrSlice(v); err == nil {
		return cleanPolicyMapStrSlice(list, filters)
	}
	return nil, fmt.Errorf("expected map or list, found %T", v)
}

// applyElementsEntriesCleaning applies filters to each element of the list stored at key.
func applyElementsEntriesCleaning(policyMap common.MapStr, key string, v any, filters []policyEntryFilter) error {
	list, err := common.ToMapStrSlice(v)
	if err != nil {
		return err
	}
	clean, err := cleanPolicyMapStrSlice(list, filters)
	if err != nil {
		return err
	}
	return replaceMapStrValue(policyMap, key, clean)
}

// applyMapValuesCleaning recurses into each value of the nested map stored in v,
// applying filters to every child (whether map or slice).
func applyMapValuesCleaning(v any, filters []policyEntryFilter) error {
	mapStr, err := common.ToMapStr(v)
	if err != nil {
		return err
	}
	for k, child := range mapStr {
		cleaned, err := cleanNestedPolicyValue(child, filters)
		if err != nil {
			return err
		}
		if err := replaceMapStrValue(mapStr, k, cleaned); err != nil {
			return err
		}
	}
	return nil
}

// applyMemberReplace applies the regexp replacement to keys (for MapStr) or string elements (for []any).
func applyMemberReplace(policyMap common.MapStr, key string, v any, r *policyEntryReplace) error {
	switch val := v.(type) {
	case common.MapStr:
		for k, e := range val {
			if r.regexp.MatchString(k) {
				delete(val, k)
				val[r.regexp.ReplaceAllString(k, r.replace)] = e
			}
		}
		return nil
	case []any:
		replaced, err := mapStringElemsInAnySlice(val, func(s string) string {
			if r.regexp.MatchString(s) {
				return r.regexp.ReplaceAllString(s, r.replace)
			}
			return s
		})
		if err != nil {
			return err
		}
		return replaceMapStrValue(policyMap, key, replaced)
	default:
		return fmt.Errorf("expected map or array for memberReplace, found %T", v)
	}
}

// applyStringValueReplace applies the regexp replacement to a single string value at key.
func applyStringValueReplace(policyMap common.MapStr, key string, v any, r *policyEntryReplace) error {
	vStr, ok := v.(string)
	if !ok {
		return fmt.Errorf("expected string, found %T", v)
	}
	if r.regexp.MatchString(vStr) {
		policyMap.Put(key, r.regexp.ReplaceAllString(vStr, r.replace)) //nolint:errcheck // single string Put cannot fail
	}
	return nil
}

// applyDeletePattern removes matching keys (for MapStr) or matching string elements (for []any).
func applyDeletePattern(policyMap common.MapStr, key string, v any, pattern *regexp.Regexp) error {
	switch val := v.(type) {
	case common.MapStr:
		for k := range val {
			if pattern.MatchString(k) {
				delete(val, k)
			}
		}
		return nil
	case []any:
		filtered, err := filterStringElemsInAnySlice(val, func(s string) bool {
			return !pattern.MatchString(s)
		})
		if err != nil {
			return err
		}
		return replaceMapStrValue(policyMap, key, filtered)
	default:
		return fmt.Errorf("expected map or array for deletePattern, found %T", v)
	}
}

// mapStringElemsInAnySlice applies f to every string element of val.
// Returns an error if any element is not a string.
func mapStringElemsInAnySlice(val []any, f func(string) string) ([]any, error) {
	out := make([]any, len(val))
	for i, elem := range val {
		s, ok := elem.(string)
		if !ok {
			return nil, fmt.Errorf("expected string array element, found %T", elem)
		}
		out[i] = f(s)
	}
	return out, nil
}

// filterStringElemsInAnySlice returns a new slice containing only elements for which keep returns true.
// Returns an error if any element is not a string.
func filterStringElemsInAnySlice(val []any, keep func(string) bool) ([]any, error) {
	filtered := val[:0]
	for _, elem := range val {
		s, ok := elem.(string)
		if !ok {
			return nil, fmt.Errorf("expected string array element, found %T", elem)
		}
		if keep(s) {
			filtered = append(filtered, s)
		}
	}
	return filtered, nil
}

// isEmpty checks if the value is empty. It is considered empty if it is the zero value,
// or for values for length, if it is zero. Values in ignoreValues are not counted for
// the total length when present in lists.
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
