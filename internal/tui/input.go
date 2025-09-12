// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// Input represents a text input prompt
type Input struct {
	message      string
	defaultValue string
	value        string
	cursor       int
	focused      bool
	error        string
}

// NewInput creates a new input prompt
func NewInput(message, defaultValue string) *Input {
	return &Input{
		message:      message,
		defaultValue: defaultValue,
		value:        "", // Start with empty value
		focused:      true,
	}
}

func (i *Input) Message() string         { return i.message }
func (i *Input) Default() interface{}    { return i.defaultValue }
func (i *Input) SetError(err string)     { i.error = err }
func (i *Input) SetFocused(focused bool) { i.focused = focused }

// Value returns the current value or default if empty
func (i *Input) Value() interface{} {
	if strings.TrimSpace(i.value) == "" && i.defaultValue != "" {
		return i.defaultValue
	}
	return i.value
}

func (i *Input) Update(msg tea.Msg) (Prompt, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "left":
			if i.cursor > 0 {
				i.cursor--
			}
		case "right":
			if i.cursor < len(i.value) {
				i.cursor++
			}
		case "home":
			i.cursor = 0
		case "end":
			i.cursor = len(i.value)
		case "backspace":
			if i.cursor > 0 {
				i.value = i.value[:i.cursor-1] + i.value[i.cursor:]
				i.cursor--
			}
		case "delete":
			if i.cursor < len(i.value) {
				i.value = i.value[:i.cursor] + i.value[i.cursor+1:]
			}
		default:
			if len(msg.String()) == 1 {
				i.value = i.value[:i.cursor] + msg.String() + i.value[i.cursor:]
				i.cursor++
			}
		}
	}
	return i, nil
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
	displayValue := i.value
	if i.focused && i.cursor <= len(displayValue) {
		// Add cursor
		if i.cursor == len(displayValue) {
			displayValue += "_"
		} else {
			displayValue = displayValue[:i.cursor] + "_" + displayValue[i.cursor+1:]
		}
	}

	if i.focused {
		b.WriteString(focusedStyle.Render("> " + displayValue))
	} else {
		b.WriteString(blurredStyle.Render("  " + displayValue))
	}

	// Error message
	if i.error != "" {
		b.WriteString("\n")
		b.WriteString(errorStyle.Render("✗ " + i.error))
	}

	return b.String()
}
