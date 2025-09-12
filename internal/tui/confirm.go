// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// Confirm represents a yes/no confirmation prompt
type Confirm struct {
	message      string
	defaultValue bool
	value        bool
	focused      bool
	error        string
}

// NewConfirm creates a new confirm prompt
func NewConfirm(message string, defaultValue bool) *Confirm {
	return &Confirm{
		message:      message,
		defaultValue: defaultValue,
		value:        defaultValue,
		focused:      true,
	}
}

func (c *Confirm) Message() string         { return c.message }
func (c *Confirm) Default() interface{}    { return c.defaultValue }
func (c *Confirm) Value() interface{}      { return c.value }
func (c *Confirm) SetError(err string)     { c.error = err }
func (c *Confirm) SetFocused(focused bool) { c.focused = focused }

func (c *Confirm) Update(msg tea.Msg) (Prompt, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch strings.ToLower(msg.String()) {
		case "y":
			c.value = true
		case "n":
			c.value = false
		case "left", "right":
			c.value = !c.value
		}
	}
	return c, nil
}

func (c *Confirm) Render() string {
	var b strings.Builder

	// Question message
	style := blurredStyle
	if c.focused {
		style = focusedStyle
	}
	b.WriteString(style.Render(c.message))

	defaultText := "N/y"
	if c.defaultValue {
		defaultText = "Y/n"
	}
	b.WriteString(helpStyle.Render(fmt.Sprintf(" (%s)", defaultText)))
	b.WriteString("\n")

	// Current selection
	yesStyle := blurredStyle
	noStyle := blurredStyle

	if c.focused {
		if c.value {
			yesStyle = focusedStyle
		} else {
			noStyle = focusedStyle
		}
	}

	b.WriteString("  ")
	b.WriteString(yesStyle.Render("Yes"))
	b.WriteString(" / ")
	b.WriteString(noStyle.Render("No"))

	// Error message
	if c.error != "" {
		b.WriteString("\n")
		b.WriteString(errorStyle.Render("✗ " + c.error))
	}

	return b.String()
}
