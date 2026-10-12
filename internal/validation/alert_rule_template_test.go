// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package validation

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/require"
)

const validV2Template = `{
  "id": "test-v2",
  "type": "alerting_rule_template",
  "managed": true,
  "attributes": {
    "engine": "v2",
    "rule": {
      "kind": "alert",
      "recovery": {"strategy": "no_breach"},
      "no_data": {"strategy": "ignore"},
      "metadata": {"name": "Test", "description": "A test rule", "tags": ["Test"]},
      "schedule": {"every": "1m", "lookback": "5m"},
      "time_field": "@timestamp",
      "query": {
        "base": "FROM metrics | STATS value = MAX(value) BY host.name",
        "breach": {"segment": "WHERE value > 1"}
      },
      "state_transition": {"pending": {"count": 2}, "recovering": {"count": 0}},
      "grouping": {"fields": ["host.name"]},
      "artifacts": [{"id": "test-v2-runbook", "type": "runbook", "data": {"content": "Investigate."}}]
    }
  }
}`

const validSignalTemplate = `{
  "id": "test-signal",
  "type": "alerting_rule_template",
  "attributes": {
    "engine": "v2",
    "rule": {
      "kind": "signal",
      "metadata": {"name": "Signal"},
      "schedule": {"every": "5s"},
      "query": {"base": "FROM logs | WHERE event.action == \"start\""}
    }
  }
}`

func TestV2TemplateStructure(t *testing.T) {
	require.NoError(t, validateAlertRuleTemplate([]byte(validV2Template)))
	require.NoError(t, validateAlertRuleTemplate([]byte(validSignalTemplate)))
	require.NoError(t, validateAlertRuleTemplate([]byte(`{"attributes":{"name":"Classic"}}`)))
	require.NoError(t, validateAlertRuleTemplate([]byte(`{"id":"future","type":"alerting_rule_template","attributes":{"engine":"v2","future":true,"rule":{"kind":"alert","future_option":true,"metadata":{"name":"Future"},"schedule":{"every":"1m"},"query":{"base":"FROM logs"},"recovery":{"strategy":"no_breach"},"no_data":{"strategy":"ignore"}}}}`)))

	rule := func(doc map[string]any) map[string]any {
		return doc["attributes"].(map[string]any)["rule"].(map[string]any)
	}
	tests := []struct {
		name string
		edit func(map[string]any)
		want string
	}{
		{"kind", func(doc map[string]any) { rule(doc)["kind"] = "not-a-valid-rule-kind" }, "rule.kind"},
		{"metadata", func(doc map[string]any) { delete(rule(doc), "metadata") }, "metadata"},
		{"schedule", func(doc map[string]any) { delete(rule(doc), "schedule") }, "schedule"},
		{"duration", func(doc map[string]any) { rule(doc)["schedule"] = map[string]any{"every": "yesterday"} }, "schedule.every"},
		{"schedule below minimum", func(doc map[string]any) { rule(doc)["schedule"] = map[string]any{"every": "1s"} }, "schedule.every"},
		{"query", func(doc map[string]any) { delete(rule(doc), "query") }, "rule.query"},
		{"removed format", func(doc map[string]any) {
			rule(doc)["query"].(map[string]any)["format"] = "composed"
		}, "query.format"},
		{"base", func(doc map[string]any) { rule(doc)["query"] = map[string]any{"base": " "} }, "query.base"},
		{"breach query field", func(doc map[string]any) {
			rule(doc)["query"].(map[string]any)["breach"] = map[string]any{"query": "ROW x=1"}
		}, "use segment"},
		{"missing recovery", func(doc map[string]any) { delete(rule(doc), "recovery") }, "require recovery"},
		{"missing no_data", func(doc map[string]any) { delete(rule(doc), "no_data") }, "require no_data"},
		{"condition without breach", func(doc map[string]any) {
			query := rule(doc)["query"].(map[string]any)
			delete(query, "breach")
			rule(doc)["recovery"] = map[string]any{"strategy": "condition", "segment": "WHERE value < 1"}
		}, "requires query.breach"},
		{"manual recovering", func(doc map[string]any) {
			rule(doc)["recovery"] = map[string]any{"strategy": "manual"}
		}, "cannot set state_transition.recovering"},
		{"signal recovery", func(doc map[string]any) {
			rule(doc)["kind"] = "signal"
			delete(rule(doc), "state_transition")
		}, "signal rules cannot set recovery"},
		{"negative delay", func(doc map[string]any) {
			rule(doc)["state_transition"] = map[string]any{"pending": map[string]any{"count": -1}}
		}, "must be an integer"},
		{"fractional delay", func(doc map[string]any) {
			rule(doc)["state_transition"] = map[string]any{"pending": map[string]any{"count": 1.5}}
		}, "must be an integer"},
		{"group shape", func(doc map[string]any) { rule(doc)["grouping"] = map[string]any{"fields": "host"} }, "must be an array"},
		{"group missing", func(doc map[string]any) { rule(doc)["grouping"] = map[string]any{} }, "must be an array"},
		{"empty field", func(doc map[string]any) {
			rule(doc)["grouping"] = map[string]any{"fields": []string{""}}
		}, "empty or oversized"},
		{"empty groups allowed", func(doc map[string]any) {
			rule(doc)["grouping"] = map[string]any{"fields": []string{}}
		}, ""},
		{"dashboard camel case", func(doc map[string]any) {
			rule(doc)["artifacts"] = []any{map[string]any{"id": "dash", "type": "dashboard", "data": map[string]any{"dashboardId": "abc"}}}
		}, "dashboard_id"},
		{"query syntax needs stack", func(doc map[string]any) {
			rule(doc)["query"].(map[string]any)["base"] = "THIS IS NOT ESQL"
		}, ""},
		{"unsupported engine", func(doc map[string]any) { doc["attributes"].(map[string]any)["engine"] = "v3" }, "unsupported alert template engine"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var doc map[string]any
			require.NoError(t, json.Unmarshal([]byte(validV2Template), &doc))
			tt.edit(doc)
			raw, err := json.Marshal(doc)
			require.NoError(t, err)
			err = validateAlertRuleTemplate(raw)
			if tt.want == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tt.want)
			}
		})
	}
}

func TestV2TemplateStringLimits(t *testing.T) {
	fields := []struct {
		name  string
		limit int
		set   func(map[string]any, string)
	}{
		{"id", 150, func(doc map[string]any, value string) { doc["id"] = value }},
		{"name", 256, func(doc map[string]any, value string) { templateRule(doc)["metadata"].(map[string]any)["name"] = value }},
		{"description", 1024, func(doc map[string]any, value string) {
			templateRule(doc)["metadata"].(map[string]any)["description"] = value
		}},
		{"tag", 128, func(doc map[string]any, value string) {
			templateRule(doc)["metadata"].(map[string]any)["tags"] = []string{value}
		}},
		{"time_field", 256, func(doc map[string]any, value string) { templateRule(doc)["time_field"] = value }},
		{"base", 10000, func(doc map[string]any, value string) { templateRule(doc)["query"].(map[string]any)["base"] = value }},
		{"breach", 10000, func(doc map[string]any, value string) {
			templateRule(doc)["query"].(map[string]any)["breach"] = map[string]any{"segment": value}
		}},
		{"grouping", 256, func(doc map[string]any, value string) {
			templateRule(doc)["grouping"] = map[string]any{"fields": []string{value}}
		}},
		{"runbook", 50000, func(doc map[string]any, value string) {
			templateRule(doc)["artifacts"] = []any{map[string]any{"id": "runbook", "type": "runbook", "data": map[string]any{"content": value}}}
		}},
		{"dashboard", 1024, func(doc map[string]any, value string) {
			templateRule(doc)["artifacts"] = []any{map[string]any{"id": "dashboard", "type": "dashboard", "data": map[string]any{"dashboard_id": value}}}
		}},
	}
	for _, field := range fields {
		for _, character := range []string{"a", "é", "😀"} {
			// JavaScript string limits count an astral character as two UTF-16 units.
			count := field.limit
			if character == "😀" {
				count /= 2
			}
			t.Run(field.name+"/"+character, func(t *testing.T) {
				var doc map[string]any
				require.NoError(t, json.Unmarshal([]byte(validV2Template), &doc))
				field.set(doc, strings.Repeat(character, count))
				raw, err := json.Marshal(doc)
				require.NoError(t, err)
				require.NoError(t, validateAlertRuleTemplate(raw))
				field.set(doc, strings.Repeat(character, count+1))
				raw, err = json.Marshal(doc)
				require.NoError(t, err)
				require.Error(t, validateAlertRuleTemplate(raw))
			})
		}
	}
}

func TestV2TemplateDurationBounds(t *testing.T) {
	for _, field := range []string{"lookback", "pending", "recovering"} {
		for _, duration := range []string{"0ms", "0s", "0m", "0h", "0d", "0w", "365d", "366d", "-1s", "invalid"} {
			t.Run(field+"/"+duration, func(t *testing.T) {
				var doc map[string]any
				require.NoError(t, json.Unmarshal([]byte(validV2Template), &doc))
				rule := templateRule(doc)
				if field == "lookback" {
					rule["schedule"].(map[string]any)[field] = duration
				} else {
					rule["state_transition"].(map[string]any)[field] = map[string]any{"timeframe": duration}
				}
				raw, err := json.Marshal(doc)
				require.NoError(t, err)
				err = validateAlertRuleTemplate(raw)
				if duration == "366d" || duration == "-1s" || duration == "invalid" {
					require.Error(t, err)
				} else {
					require.NoError(t, err)
				}
			})
		}
	}
	var doc map[string]any
	require.NoError(t, json.Unmarshal([]byte(validV2Template), &doc))
	templateRule(doc)["schedule"].(map[string]any)["every"] = "0s"
	raw, err := json.Marshal(doc)
	require.NoError(t, err)
	require.ErrorContains(t, validateAlertRuleTemplate(raw), "schedule.every")
}

func TestV2TemplateNoDataQuery(t *testing.T) {
	queries := []struct {
		name  string
		value any
		valid bool
	}{
		{"query", "FROM metrics", true},
		{"syntax needs stack", "THIS IS NOT ESQL", true},
		{"unicode at limit", strings.Repeat("é", 10000), true},
		{"oversized", strings.Repeat("a", 10001), false},
		{"empty", "", false},
		{"whitespace", " \n ", false},
		{"null", nil, false},
		{"object", map[string]any{}, false},
		{"array", []string{}, false},
		{"number", 1, false},
	}
	for _, strategy := range []string{"keep_last", "resolve"} {
		for _, hasBreach := range []bool{false, true} {
			for _, query := range queries {
				t.Run(fmt.Sprintf("%s/breach=%t/%s", strategy, hasBreach, query.name), func(t *testing.T) {
					var doc map[string]any
					require.NoError(t, json.Unmarshal([]byte(validV2Template), &doc))
					rule := templateRule(doc)
					if !hasBreach {
						delete(rule["query"].(map[string]any), "breach")
					}
					rule["no_data"] = map[string]any{"strategy": strategy, "query": query.value}
					raw, err := json.Marshal(doc)
					require.NoError(t, err)
					err = validateAlertRuleTemplate(raw)
					if query.valid {
						require.NoError(t, err)
					} else {
						require.ErrorContains(t, err, "no_data.query")
					}
				})
			}
		}
	}
}

func TestV2TemplateTransitionCounts(t *testing.T) {
	for _, phase := range []string{"pending", "recovering"} {
		for _, count := range []string{"0", "-0", "2", "2.0", "2e0", "2e1", "1000.0", "-1", "1.5", "1e-1", "1001", "1e999", `"2"`, "true", "[]", "{}"} {
			t.Run(phase+"/"+count, func(t *testing.T) {
				var doc map[string]any
				require.NoError(t, json.Unmarshal([]byte(validV2Template), &doc))
				// Keep the original JSON spelling; decoding and re-encoding a float
				// would otherwise normalize 2.0 and 2e0 to 2 before validation.
				templateRule(doc)["state_transition"].(map[string]any)[phase] = map[string]any{"count": json.RawMessage(count)}
				raw, err := json.Marshal(doc)
				require.NoError(t, err)
				err = validateAlertRuleTemplate(raw)
				switch count {
				case "0", "-0", "2", "2.0", "2e0", "2e1", "1000.0":
					require.NoError(t, err)
				default:
					require.ErrorContains(t, err, "must be an integer")
				}
			})
		}
	}
}

func templateRule(doc map[string]any) map[string]any {
	return doc["attributes"].(map[string]any)["rule"].(map[string]any)
}

func TestV2TemplateOptionalNulls(t *testing.T) {
	paths := [][]string{
		{"metadata", "description"}, {"metadata", "tags"},
		{"schedule", "lookback"}, {"time_field"}, {"query", "breach"},
		{"query", "format"}, {"grouping"}, {"artifacts"},
		{"state_transition", "pending"}, {"state_transition", "recovering"},
		{"state_transition", "pending", "count"}, {"state_transition", "pending", "timeframe"},
		{"state_transition", "pending", "operator"},
		{"state_transition", "recovering", "count"}, {"state_transition", "recovering", "timeframe"},
		{"state_transition", "recovering", "operator"},
		{"recovery", "query"}, {"recovery", "segment"}, {"no_data", "query"},
	}
	for _, path := range paths {
		t.Run(strings.Join(path, "."), func(t *testing.T) {
			var doc map[string]any
			require.NoError(t, json.Unmarshal([]byte(validV2Template), &doc))
			rule := templateRule(doc)
			// Each phase remains valid when either threshold is omitted.
			for _, phase := range []string{"pending", "recovering"} {
				rule["state_transition"].(map[string]any)[phase].(map[string]any)["timeframe"] = "1m"
			}
			parent := rule
			for _, field := range path[:len(path)-1] {
				parent = parent[field].(map[string]any)
			}
			field := path[len(path)-1]
			delete(parent, field)
			raw, err := json.Marshal(doc)
			require.NoError(t, err)
			require.NoError(t, validateAlertRuleTemplate(raw), "omission should be accepted")
			parent[field] = nil
			raw, err = json.Marshal(doc)
			require.NoError(t, err)
			require.Error(t, validateAlertRuleTemplate(raw), "explicit null should be rejected")
		})
	}
	for _, template := range []string{validV2Template, validSignalTemplate} {
		var doc map[string]any
		require.NoError(t, json.Unmarshal([]byte(template), &doc))
		templateRule(doc)["state_transition"] = nil
		raw, err := json.Marshal(doc)
		require.NoError(t, err)
		require.NoError(t, validateAlertRuleTemplate(raw), "state_transition itself is nullable")
	}
}

func TestV2SignalTransitions(t *testing.T) {
	for _, transition := range []string{"missing", "null", "{}", `{"pending":{"count":0}}`, "[]", "2", "false", `"invalid"`} {
		t.Run(transition, func(t *testing.T) {
			var doc map[string]any
			require.NoError(t, json.Unmarshal([]byte(validSignalTemplate), &doc))
			if transition != "missing" {
				templateRule(doc)["state_transition"] = json.RawMessage(transition)
			}
			raw, err := json.Marshal(doc)
			require.NoError(t, err)
			err = validateAlertRuleTemplate(raw)
			if transition == "missing" || transition == "null" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, "signal rules cannot set state_transition")
			}
		})
	}
	for _, field := range []string{"recovery", "no_data"} {
		t.Run(field+"/null", func(t *testing.T) {
			var doc map[string]any
			require.NoError(t, json.Unmarshal([]byte(validSignalTemplate), &doc))
			templateRule(doc)[field] = nil
			raw, err := json.Marshal(doc)
			require.NoError(t, err)
			require.ErrorContains(t, validateAlertRuleTemplate(raw), "signal rules cannot set recovery or no_data")
		})
	}
}

func TestV2TemplateValidationEntryPoints(t *testing.T) {
	tests := []struct {
		name     string
		template string
		edit     func(map[string]any)
		want     string
	}{
		{"valid", validV2Template, func(rule map[string]any) {}, ""},
		{"nullable transition", validV2Template, func(rule map[string]any) { rule["state_transition"] = nil }, ""},
		{"signal array", validSignalTemplate, func(rule map[string]any) { rule["state_transition"] = []any{} }, "state_transition"},
		{"null description", validV2Template, func(rule map[string]any) { rule["metadata"].(map[string]any)["description"] = nil }, "metadata.description"},
		{"null lookback", validV2Template, func(rule map[string]any) { rule["schedule"].(map[string]any)["lookback"] = nil }, "schedule.lookback"},
		{"null count", validV2Template, func(rule map[string]any) {
			rule["state_transition"].(map[string]any)["pending"] = map[string]any{"count": nil, "timeframe": "1m"}
		}, "state_transition.pending.count"},
	}
	validators := []struct {
		name     string
		fixture  string
		validate func(string) (error, error)
	}{
		{"source", sourcePackageDir, ValidateSourceFromPath},
		{"built", builtPackageDir, ValidateBuiltFromPath},
		{"zip", builtPackageDir, func(root string) (error, error) { return ValidateBuiltFromZip(makeZipFromDir(t, root)) }},
	}
	for _, test := range tests {
		for _, validator := range validators {
			t.Run(test.name+"/"+validator.name, func(t *testing.T) {
				root := t.TempDir()
				require.NoError(t, os.CopyFS(root, os.DirFS(validator.fixture)))
				var doc map[string]any
				require.NoError(t, json.Unmarshal([]byte(test.template), &doc))
				test.edit(templateRule(doc))
				raw, err := json.Marshal(doc)
				require.NoError(t, err)
				dir := filepath.Join(root, "kibana", "alerting_rule_template")
				require.NoError(t, os.MkdirAll(dir, 0o755))
				require.NoError(t, os.WriteFile(filepath.Join(dir, doc["id"].(string)+".json"), raw, 0o644))
				err, removed := validator.validate(root)
				require.NoError(t, removed)
				if test.want == "" {
					require.NoError(t, err)
				} else {
					require.ErrorContains(t, err, test.want)
				}
			})
		}
	}
}

func TestIssueReproductionIsStructuralNotQueryParsing(t *testing.T) {
	invalid := []byte(`{
	  "id": "invalid-v2-example",
	  "type": "alerting_rule_template",
	  "attributes": {
	    "engine": "v2",
	    "rule": {
	      "metadata": {"name": "Invalid v2 example", "description": "Build validation reproduction"},
	      "kind": "not-a-valid-rule-kind",
	      "query": {"format": "composed", "base": "THIS IS NOT ESQL"}
	    }
	  }
	}`)
	err := validateAlertRuleTemplate(invalid)
	require.ErrorContains(t, err, "rule.kind")
	require.ErrorContains(t, err, "rule.schedule")
	require.ErrorContains(t, err, "query.format")
	require.NotContains(t, err.Error(), "Invalid ES|QL")
}

func TestV2ValidationPathAndZip(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "kibana", "alerting_rule_template")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	invalid := []byte(`{"id":"bad","type":"alerting_rule_template","attributes":{"engine":"v2","rule":{"kind":"invalid"}}}`)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "bad.json"), invalid, 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "validation.yml"), []byte("errors:\n  exclude_checks:\n    - SVR00001\n    - SVR00002\n"), 0o644))

	errs, _ := ValidateSourceFromPath(root)
	require.ErrorContains(t, errs, "rule.kind")
	errs, _ = ValidateBuiltFromPath(root)
	require.ErrorContains(t, errs, "rule.kind")

	goodRoot := t.TempDir()
	goodDir := filepath.Join(goodRoot, "kibana", "alerting_rule_template")
	require.NoError(t, os.MkdirAll(goodDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(goodDir, "good.json"), []byte(validV2Template), 0o644))
	errs, _ = ValidateSourceFromPath(goodRoot)
	if errs != nil {
		require.NotContains(t, errs.Error(), "invalid v2 alert rule template")
	}

	filename := filepath.Join(t.TempDir(), "package.zip")
	f, err := os.Create(filename)
	require.NoError(t, err)
	w := zip.NewWriter(f)
	entry, err := w.Create("test-1.0.0/kibana/alerting_rule_template/bad.json")
	require.NoError(t, err)
	_, err = entry.Write(invalid)
	require.NoError(t, err)
	require.NoError(t, w.Close())
	require.NoError(t, f.Close())
	errs, _ = ValidateBuiltFromZip(filename)
	require.ErrorContains(t, errs, "rule.kind")

	require.NoError(t, validateAlertRuleTemplates(fstest.MapFS{}))
	require.NoError(t, validateAlertRuleTemplates(fstest.MapFS{
		"kibana/alerting_rule_template/good.json": {Data: []byte(validV2Template)},
	}))
}
