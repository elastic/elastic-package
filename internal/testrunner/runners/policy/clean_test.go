// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package policy

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/elastic-package/internal/common"
)

func TestReplaceMapStrValue(t *testing.T) {
	t.Run("replaces existing scalar value", func(t *testing.T) {
		m := common.MapStr{"key": "old"}
		err := replaceMapStrValue(m, "key", "new")
		require.NoError(t, err)
		assert.Equal(t, "new", m["key"])
	})

	t.Run("replaces existing slice value", func(t *testing.T) {
		m := common.MapStr{"key": []any{"a", "b"}}
		err := replaceMapStrValue(m, "key", []any{"x"})
		require.NoError(t, err)
		assert.Equal(t, []any{"x"}, m["key"])
	})

	t.Run("sets a new key", func(t *testing.T) {
		m := common.MapStr{}
		err := replaceMapStrValue(m, "new_key", 42)
		require.NoError(t, err)
		assert.Equal(t, 42, m["new_key"])
	})
}

func TestMapStringElemsInAnySlice(t *testing.T) {
	t.Run("transforms all string elements", func(t *testing.T) {
		in := []any{"hello", "world"}
		out, err := mapStringElemsInAnySlice(in, func(s string) string { return s + "!" })
		require.NoError(t, err)
		assert.Equal(t, []any{"hello!", "world!"}, out)
	})

	t.Run("returns error on non-string element", func(t *testing.T) {
		in := []any{"ok", 42}
		_, err := mapStringElemsInAnySlice(in, func(s string) string { return s })
		require.Error(t, err)
		assert.Contains(t, err.Error(), "expected string array element")
	})

	t.Run("empty slice returns empty slice", func(t *testing.T) {
		out, err := mapStringElemsInAnySlice([]any{}, func(s string) string { return s })
		require.NoError(t, err)
		assert.Empty(t, out)
	})
}

func TestFilterStringElemsInAnySlice(t *testing.T) {
	t.Run("keeps only elements matching predicate", func(t *testing.T) {
		in := []any{"keep", "drop", "keep2"}
		out, err := filterStringElemsInAnySlice(in, func(s string) bool { return s != "drop" })
		require.NoError(t, err)
		assert.Equal(t, []any{"keep", "keep2"}, out)
	})

	t.Run("returns error on non-string element", func(t *testing.T) {
		in := []any{"ok", 123}
		_, err := filterStringElemsInAnySlice(in, func(s string) bool { return true })
		require.Error(t, err)
		assert.Contains(t, err.Error(), "expected string array element")
	})

	t.Run("returns empty when all filtered out", func(t *testing.T) {
		in := []any{"a", "b"}
		out, err := filterStringElemsInAnySlice(in, func(s string) bool { return false })
		require.NoError(t, err)
		assert.Empty(t, out)
	})
}

func TestApplyElementsEntriesCleaning(t *testing.T) {
	t.Run("removes the named field from each list element", func(t *testing.T) {
		m := common.MapStr{
			"items": []any{
				common.MapStr{"id": "abc", "name": "foo"},
				common.MapStr{"id": "def", "name": "bar"},
			},
		}
		v, _ := m.GetValue("items")
		err := applyElementsEntriesCleaning(m, "items", v, []policyEntryFilter{{name: "id"}})
		require.NoError(t, err)

		items, err := common.ToMapStrSlice(m["items"])
		require.NoError(t, err)
		for _, item := range items {
			_, hasID := item["id"]
			assert.False(t, hasID, "id should have been removed")
			_, hasName := item["name"]
			assert.True(t, hasName, "name should be preserved")
		}
	})

	t.Run("returns error when value is not a slice of maps", func(t *testing.T) {
		m := common.MapStr{"items": "not-a-list"}
		err := applyElementsEntriesCleaning(m, "items", "not-a-list", []policyEntryFilter{{name: "id"}})
		require.Error(t, err)
	})
}

func TestApplyMapValuesCleaning(t *testing.T) {
	t.Run("removes filtered field from each nested map value", func(t *testing.T) {
		m := common.MapStr{
			"section": common.MapStr{
				"alpha": common.MapStr{"auth": "secret", "url": "http://example.com"},
				"beta":  common.MapStr{"auth": "secret2", "url": "http://other.com"},
			},
		}
		v, _ := m.GetValue("section")
		err := applyMapValuesCleaning(v, []policyEntryFilter{{name: "auth"}})
		require.NoError(t, err)

		section, err := common.ToMapStr(m["section"])
		require.NoError(t, err)
		for _, child := range section {
			childMap, err := common.ToMapStr(child)
			require.NoError(t, err)
			_, hasAuth := childMap["auth"]
			assert.False(t, hasAuth, "auth should have been removed")
			_, hasURL := childMap["url"]
			assert.True(t, hasURL, "url should be preserved")
		}
	})

	t.Run("returns error when value is not a map", func(t *testing.T) {
		err := applyMapValuesCleaning("not-a-map", []policyEntryFilter{{name: "id"}})
		require.Error(t, err)
	})
}

func TestApplyMemberReplace(t *testing.T) {
	re := &policyEntryReplace{
		regexp:  regexp.MustCompile(`^uuid-.*$`),
		replace: "normalized",
	}

	t.Run("replaces matching keys in a MapStr", func(t *testing.T) {
		m := common.MapStr{
			"perms": common.MapStr{
				"uuid-abc-123": "value1",
				"_keep":        "value2",
			},
		}
		v, _ := m.GetValue("perms")
		err := applyMemberReplace(m, "perms", v, re)
		require.NoError(t, err)

		perms, err := common.ToMapStr(m["perms"])
		require.NoError(t, err)
		_, hasOld := perms["uuid-abc-123"]
		assert.False(t, hasOld, "original key should be gone")
		_, hasNew := perms["normalized"]
		assert.True(t, hasNew, "replacement key should exist")
		_, hasKept := perms["_keep"]
		assert.True(t, hasKept, "non-matching key should be preserved")
	})

	t.Run("replaces matching string elements in a []any", func(t *testing.T) {
		m := common.MapStr{
			"refs": []any{"uuid-abc-123", "keep-me"},
		}
		v := m["refs"]
		err := applyMemberReplace(m, "refs", v, re)
		require.NoError(t, err)
		assert.Equal(t, []any{"normalized", "keep-me"}, m["refs"])
	})

	t.Run("returns error on non-string element in []any", func(t *testing.T) {
		m := common.MapStr{"refs": []any{42}}
		err := applyMemberReplace(m, "refs", m["refs"], re)
		require.Error(t, err)
	})

	t.Run("returns error on unexpected value type", func(t *testing.T) {
		m := common.MapStr{"refs": "a plain string"}
		err := applyMemberReplace(m, "refs", m["refs"], re)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "expected map or array for memberReplace")
	})
}

func TestApplyStringValueReplace(t *testing.T) {
	re := &policyEntryReplace{
		regexp:  regexp.MustCompile(`^(.+)-[0-9]+$`),
		replace: "$1",
	}

	t.Run("strips numeric suffix from matching string", func(t *testing.T) {
		m := common.MapStr{"name": "my-input-42"}
		err := applyStringValueReplace(m, "name", m["name"], re)
		require.NoError(t, err)
		assert.Equal(t, "my-input", m["name"])
	})

	t.Run("leaves non-matching string unchanged", func(t *testing.T) {
		m := common.MapStr{"name": "my-input"}
		err := applyStringValueReplace(m, "name", m["name"], re)
		require.NoError(t, err)
		assert.Equal(t, "my-input", m["name"])
	})

	t.Run("returns error when value is not a string", func(t *testing.T) {
		m := common.MapStr{"name": 99}
		err := applyStringValueReplace(m, "name", m["name"], re)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "expected string")
	})
}

func TestApplyDeletePattern(t *testing.T) {
	pat := regexp.MustCompile(`^beatsauth/`)

	t.Run("removes matching keys from a MapStr", func(t *testing.T) {
		m := common.MapStr{
			"extensions": common.MapStr{
				"beatsauth/default": common.MapStr{},
				"health_check/abc":  common.MapStr{},
			},
		}
		v, _ := m.GetValue("extensions")
		err := applyDeletePattern(m, "extensions", v, pat)
		require.NoError(t, err)

		ext, err := common.ToMapStr(m["extensions"])
		require.NoError(t, err)
		_, hasBeats := ext["beatsauth/default"]
		assert.False(t, hasBeats, "beatsauth key should be removed")
		_, hasHealth := ext["health_check/abc"]
		assert.True(t, hasHealth, "non-matching key should remain")
	})

	t.Run("filters matching string elements from []any", func(t *testing.T) {
		m := common.MapStr{
			"service": common.MapStr{
				"extensions": []any{"beatsauth/default", "health_check/abc"},
			},
		}
		v, _ := m.GetValue("service.extensions")
		err := applyDeletePattern(m, "service.extensions", v, pat)
		require.NoError(t, err)
		got, err := m.GetValue("service.extensions")
		require.NoError(t, err)
		assert.Equal(t, []any{"health_check/abc"}, got)
	})

	t.Run("returns error on non-string element in []any", func(t *testing.T) {
		m := common.MapStr{"list": []any{123}}
		err := applyDeletePattern(m, "list", m["list"], pat)
		require.Error(t, err)
	})

	t.Run("returns error on unexpected value type", func(t *testing.T) {
		m := common.MapStr{"field": "scalar"}
		err := applyDeletePattern(m, "field", m["field"], pat)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "expected map or array for deletePattern")
	})
}
