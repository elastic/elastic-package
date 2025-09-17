// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package tui

import (
	"fmt"
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

// Styles for consistent UI
var (
	focusedStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	blurredStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	errorStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	helpStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	selectedStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("46")).Bold(true) // Bright green, bold
	unselectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))           // Gray
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
