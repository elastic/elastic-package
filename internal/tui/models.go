// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package tui

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Question represents a single prompt question
type Question struct {
	Name     string
	Prompt   Prompt
	Validate Validator
}

// Prompt interface for different prompt types
type Prompt interface {
	Render() string
	Update(msg tea.Msg) (Prompt, tea.Cmd)
	Value() interface{}
	Message() string
	Default() interface{}
}

// Validator function type for validation
type Validator func(interface{}) error

// ANSI 16 color constants
const (
	ansiBlack         = "0"
	ansiRed           = "1"
	ansiGreen         = "2"
	ansiYellow        = "3"
	ansiBlue          = "4"
	ansiMagenta       = "5"
	ansiCyan          = "6"
	ansiWhite         = "7"
	ansiBrightBlack   = "8" // Gray
	ansiBrightRed     = "9"
	ansiBrightGreen   = "10"
	ansiBrightYellow  = "11"
	ansiBrightBlue    = "12"
	ansiBrightMagenta = "13"
	ansiBrightCyan    = "14"
	ansiBrightWhite   = "15"
)

// colorSupported checks if color output is supported based on environment variables
func colorSupported() bool {
	// Check NO_COLOR environment variable (https://no-color.org/)
	if os.Getenv("NO_COLOR") != "" {
		return false
	}

	// Check TERM environment variable
	term := os.Getenv("TERM")
	switch {
	case term == "":
		return false
	case term == "dumb":
		return false
	default:
		// Default to supporting color for most modern terminals
		return true
	}
}

// getColor returns the color if colors are supported, empty string otherwise
func getColor(ansiColor string) lipgloss.Color {
	if !colorSupported() {
		return lipgloss.Color("")
	}
	return lipgloss.Color(ansiColor)
}

// Styles for consistent UI using ANSI 16 colors
var (
	focusedStyle    = lipgloss.NewStyle().Foreground(getColor(ansiBrightMagenta))
	blurredStyle    = lipgloss.NewStyle().Foreground(getColor(ansiBrightBlack))
	errorStyle      = lipgloss.NewStyle().Foreground(getColor(ansiBrightRed))
	helpStyle       = lipgloss.NewStyle().Foreground(getColor(ansiBrightBlack))
	selectedStyle   = lipgloss.NewStyle().Foreground(getColor(ansiBrightGreen)).Bold(true)
	unselectedStyle = lipgloss.NewStyle().Foreground(getColor(ansiBrightBlack))
	headerStyle     = lipgloss.NewStyle().Foreground(getColor(ansiBrightCyan)).Bold(true)
	footerStyle     = lipgloss.NewStyle().Foreground(getColor(ansiBrightBlack)).Italic(true)
)

// ComposeValidators combines multiple validators
func ComposeValidators(validators ...Validator) Validator {
	return func(val interface{}) error {
		for _, validator := range validators {
			if err := validator(val); err != nil {
				return err
			}
		}
		return nil
	}
}

// Required validator
func Required(val interface{}) error {
	switch v := val.(type) {
	case string:
		if strings.TrimSpace(v) == "" {
			return fmt.Errorf("this field is required")
		}
	case []string:
		if len(v) == 0 {
			return fmt.Errorf("at least one option must be selected")
		}
	}
	return nil
}
