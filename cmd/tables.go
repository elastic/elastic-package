// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"github.com/fatih/color"
	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/renderer"
	"github.com/olekukonko/tablewriter/tw"
)

var (
	// defaultTableSymbols mimicks the table symbols of tablewriter v0,
	// we could consider using v1 defaults.
	defaultTableSymbols tw.Symbols = tw.NewSymbolCustom("Default").
				WithTopLeft("+").
				WithTopMid("+").
				WithTopRight("+").
				WithBottomLeft("+").
				WithBottomMid("+").
				WithBottomRight("+").
				WithMidLeft("+").
				WithMidRight("+")

	// defaultTableConfig enables lines wrapping and limits cell width.
	defaultTableConfig = tablewriter.Config{
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

// defaultColorizedConfig returns config for the colorized renderer that mimics
// behaviour of tablewriter v0.
func defaultColorizedConfig() renderer.ColorizedConfig {
	return renderer.ColorizedConfig{
		Header: renderer.Tint{
			FG: renderer.Colors{color.Bold},
		},
		Settings:  defaultTableRendererSettings,
		Symbols:   defaultTableSymbols,
		Border:    defaultTableLinesTint,
		Separator: defaultTableLinesTint,
	}
}
