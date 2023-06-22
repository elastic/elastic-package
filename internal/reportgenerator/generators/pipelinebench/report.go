// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package benchmark

import (
	"bytes"
	"fmt"
	"html/template"

	"github.com/elastic/elastic-package/internal/reportgenerator"
)

var resultExts = map[string]bool{".json": true, ".xml": true}

const (
	resultNoChange    = ":+1:"
	resultImprovement = ":green_heart:"
	resultWorse       = ":broken_heart:"
	tpl               = `### :rocket: Benchmarks report
{{range $package, $reports := .}}
{{if hasPrintableReports $reports}}
#### Package ` + "`" + `{{$package}}` + "`" + ` {{getReportsSummary $reports}}
<details>
<summary>Expand to view</summary>

Data stream | Previous EPS | New EPS | Diff (%) | Result
----------- | ------------ | ------- | -------- | ------
{{range $reports}}{{$result := getResult .Old .Percentage}}{{if isPrintable $result}}` +
		"`" + `{{.DataStream}}` + "`" +
		` | {{.Old}} | {{.New}} | {{.Diff}} ({{if gt .Old 0.0}}{{.Percentage}}{{else}} - {{end}}%) | {{$result}}
{{end}}{{end}}</details>{{end}}
{{end}}

`
)

func (g *generator) markdownFormat(reports Reports) ([]byte, error) {
	tpl, err := getReportTpl(g.options)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	if err := tpl.Execute(&buf, reports); err != nil {
		return nil, err
	}

	if !g.options.Full {
		buf.WriteString(`To see the full report comment with ` + "`/test benchmark fullreport`\n")
	}

	return buf.Bytes(), nil
}

func getReportTpl(opts reportgenerator.ReportOptions) (*template.Template, error) {
	return template.New("result").Funcs(map[string]interface{}{
		"getResult": func(oldValue, p float64) string {
			return getResult(opts.Threshold, oldValue, p)
		},
		"isPrintable": func(result string) bool {
			return isPrintable(opts.Full, result)
		},
		"getReportsSummary": func(reports []Report) string {
			sum := map[string]int{}
			for _, r := range reports {
				sum[getResult(opts.Threshold, r.Old, r.Percentage)] += 1
			}
			return fmt.Sprintf(
				"%s(%d) %s(%d) %s(%d)",
				resultNoChange, sum[resultNoChange],
				resultImprovement, sum[resultImprovement],
				resultWorse, sum[resultWorse],
			)
		},
		"hasPrintableReports": func(reports []Report) bool {
			for _, r := range reports {
				if isPrintable(opts.Full, getResult(opts.Threshold, r.Old, r.Percentage)) {
					return true
				}
			}
			return false
		},
	}).Parse(tpl)
}

func getResult(threshold, oldValue, p float64) string {
	switch {
	default:
		fallthrough
	case oldValue == 0:
		return resultNoChange
	case p > threshold:
		return resultImprovement
	case p < 0 && p < (threshold*-1):
		return resultWorse
	}
}

func isPrintable(fullReport bool, result string) bool {
	if fullReport {
		return true
	}
	return result == resultWorse
}
