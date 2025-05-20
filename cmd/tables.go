// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"strings"

	"github.com/fatih/color"
	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/renderer"
	"github.com/olekukonko/tablewriter/tw"
)

var (
	// defaultTableConfig enables lines wrapping and limits cell width.
	defaultTableConfig = tablewriter.Config{
		Header: tw.CellConfig{
			Filter: tw.CellFilter{
				Global: headerCellFilter,
			},
		},
		Row: tw.CellConfig{
			Formatting: tw.CellFormatting{
				AutoWrap: tw.WrapNormal,
			},
			ColMaxWidths: tw.CellWidth{Global: 32},
		},
	}

	// defaultTableLinesTint removes color from table borders.
	defaultTableLinesTint = renderer.Tint{
		BG: renderer.Colors{color.Reset},
		FG: renderer.Colors{color.Reset},
	}

	// defaultTableRendererSettings enables separator between rows and columns.
	defaultTableRendererSettings = tw.Settings{
		Separators: tw.Separators{
			BetweenColumns: tw.On,
			BetweenRows:    tw.On,
		},
	}
)

var headerCellFilterReplacer = strings.NewReplacer("_", " ", ".", " ")

// headerCellFilter mimics behaviour of tablewriter v0, where some symbols where replaced
// with spaces.
func headerCellFilter(headers []string) []string {
	result := make([]string, len(headers))
	for i := range headers {
		result[i] = headerCellFilterReplacer.Replace(headers[i])
	}
	return result
}

// defaultColorizedConfig returns config for the colorized renderer that mimics
// behaviour of tablewriter v0 and sets some defaults for headers and first column.
func defaultColorizedConfig() renderer.ColorizedConfig {
	return renderer.ColorizedConfig{
		Header: renderer.Tint{
			FG: renderer.Colors{color.Bold},
		},
		Column: renderer.Tint{
			Columns: []renderer.Tint{
				{FG: renderer.Colors{color.Bold, color.FgCyan}},
			},
		},
		Settings:  defaultTableRendererSettings,
		Symbols:   tw.NewSymbols(tw.StyleRounded),
		Border:    defaultTableLinesTint,
		Separator: defaultTableLinesTint,
	}
}
