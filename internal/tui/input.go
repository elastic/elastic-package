// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// Input represents a text input prompt using bubbles textinput
type Input struct {
	message      string
	defaultValue string
	textInput    textinput.Model
	focused      bool
	error        string
}

// compile time check that Input implements Prompt interface
var _ Prompt = &Input{}

// NewInput creates a new input prompt
func NewInput(message, defaultValue string) *Input {
	ti := textinput.New()
	ti.Focus()
	ti.CharLimit = 256
	ti.Width = 50
	ti.Prompt = "> "

	return &Input{
		message:      message,
		defaultValue: defaultValue,
		textInput:    ti,
		focused:      true,
	}
}

func (i *Input) Message() string      { return i.message }
func (i *Input) Default() interface{} { return i.defaultValue }
func (i *Input) SetError(err string)  { i.error = err }
func (i *Input) SetFocused(focused bool) {
	i.focused = focused
	if focused {
		i.textInput.Focus()
	} else {
		i.textInput.Blur()
	}
}

// Value returns the current value or default if empty
func (i *Input) Value() interface{} {
	value := strings.TrimSpace(i.textInput.Value())
	if value == "" && i.defaultValue != "" {
		return i.defaultValue
	}
	return value
}

func (i *Input) Update(msg tea.Msg) (Prompt, tea.Cmd) {
	var cmd tea.Cmd
	i.textInput, cmd = i.textInput.Update(msg)
	return i, cmd
}

func (i *Input) Render() string {
	var b strings.Builder

	// Question message
	style := blurredStyle
	if i.focused {
		style = focusedStyle
	}
	b.WriteString(style.Render(i.message))

	if i.defaultValue != "" {
		b.WriteString(helpStyle.Render(fmt.Sprintf(" (%s)", i.defaultValue)))
	}
	b.WriteString("\n")

	// Input field
	b.WriteString(i.textInput.View())

	// Error message
	if i.error != "" {
		b.WriteString("\n")
		b.WriteString(errorStyle.Render("✗ " + i.error))
	}

	return b.String()
}
