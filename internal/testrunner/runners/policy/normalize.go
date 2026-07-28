// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package policy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

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
	preNormalizeBareConnectorKey(root)
	preNormalizeBareExtensionKeys(root)
	preNormalizeBareServicePipelineKeys(root)
	preNormalizePipelineForwardRefs(root)
	// Walk string elements in arrays to strip conditional "where ... == nil" OTTL
	// suffixes from set() statements.
	preNormalizeNode(root)
}

// preNormalizeBareConnectorKey renames the bare "forward" connector key to "forward/_bare"
// so it is treated as a variable key and participates in normalization. Fleet added an
// output-ID suffix in 9.4.3 (kibana#270487) producing "forward/<outputId>" keys, which
// already contain "/" and are normalized automatically. Only the bare case needs renaming
// here; distinct "forward/<outputId>" keys are left intact so policies with multiple outputs
// retain separate forward connectors.
func preNormalizeBareConnectorKey(root map[string]any) {
	connectors, ok := toMap(root["connectors"])
	if !ok {
		return
	}
	v, hasBare := connectors["forward"]
	if !hasBare {
		return
	}
	if _, taken := connectors["forward/_bare"]; !taken {
		delete(connectors, "forward")
		connectors["forward/_bare"] = v
	}
}

// preNormalizeBareExtensionKeys renames bare extension map keys (those without "/") to
// "<name>/_bare" so they participate in component-ID normalization. Fleet started suffixing
// these with a component ID in 9.5.0, so older expected files may still use bare keys.
// String references to these extensions are resolved later by resolveExtensionRefs.
func preNormalizeBareExtensionKeys(root map[string]any) {
	extensions, ok := toMap(root["extensions"])
	if !ok {
		return
	}
	renameBareKeys(extensions)
}

// preNormalizeBareServicePipelineKeys renames bare pipeline keys (e.g. "logs", "metrics")
// to "<signal>/_bare" so they participate in normalization the same way suffixed keys do.
// Fleet started suffixing these with the output ID in 9.4.3 (kibana#270487).
func preNormalizeBareServicePipelineKeys(root map[string]any) {
	service, ok := toMap(root["service"])
	if !ok {
		return
	}
	pipelines, ok := toMap(service["pipelines"])
	if !ok {
		return
	}
	renameBareKeys(pipelines)
}

// renameBareKeys renames entries in m that don't contain "/" to "<key>/_bare",
// skipping the rename when the target already exists.
func renameBareKeys(m map[string]any) {
	for k, v := range m {
		if strings.Contains(k, "/") {
			continue
		}
		target := k + "/_bare"
		if _, taken := m[target]; !taken {
			delete(m, k)
			m[target] = v
		}
	}
}

// preNormalizePipelineForwardRefs renames bare "forward" connector refs to "forward/_bare"
// in pipeline receiver and exporter lists. Scoped to service.pipelines.*.{receivers,exporters}
// to avoid incorrectly rewriting the string "forward" that appears as an arbitrary value in
// component config lists (e.g. a filter processor body-match list).
func preNormalizePipelineForwardRefs(root map[string]any) {
	service, ok := toMap(root["service"])
	if !ok {
		return
	}
	pipelines, ok := toMap(service["pipelines"])
	if !ok {
		return
	}
	for _, p := range pipelines {
		pipeline, ok := toMap(p)
		if !ok {
			continue
		}
		for _, field := range []string{"receivers", "exporters"} {
			list, ok := pipeline[field].([]any)
			if !ok {
				continue
			}
			for i, v := range list {
				if s, ok := v.(string); ok && s == "forward" {
					list[i] = "forward/_bare"
				}
			}
		}
	}
}

// preNormalizeNode recursively walks the tree and strips conditional "where ... == nil"
// OTTL suffixes from string elements inside slices. Map keys are left to the later
// normalization pass. Extension string references are NOT touched here — they are resolved
// at known structural positions by resolveExtensionRefs.
func preNormalizeNode(node any) {
	switch n := node.(type) {
	case map[string]any:
		for _, v := range n {
			preNormalizeNode(v)
		}
	case []any:
		for i, elem := range n {
			if s, ok := elem.(string); ok {
				n[i] = ottlConditionalDataStreamAttr.ReplaceAllString(s, "")
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
	extMap, ok := toMap(root["extensions"])
	if !ok {
		return
	}
	typeToCanonical := buildExtensionTypeMapping(extMap, idMapping)
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
func buildExtensionTypeMapping(extensions map[string]any, idMapping map[string]string) map[string]string {
	typeCounts := make(map[string]int)
	for k := range extensions {
		typ, _, hasSlash := strings.Cut(k, "/")
		if hasSlash && typ != "" {
			typeCounts[typ]++
		}
	}

	typeToCanonical := make(map[string]string)
	for k := range extensions {
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
