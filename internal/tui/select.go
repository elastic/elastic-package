// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// Select represents a single-choice selection prompt
type Select struct {
	message      string
	options      []string
	defaultValue string
	selected     int
	focused      bool
	error        string
	description  func(string, int) string
}

// NewSelect creates a new select prompt
func NewSelect(message string, options []string, defaultValue string) *Select {
	selected := 0
	for i, opt := range options {
		if opt == defaultValue {
			selected = i
			break
		}
	}

	return &Select{
		message:      message,
		options:      options,
		defaultValue: defaultValue,
		selected:     selected,
		focused:      true,
	}
}

func (s *Select) Message() string                            { return s.message }
func (s *Select) Default() interface{}                       { return s.defaultValue }
func (s *Select) Value() interface{}                         { return s.options[s.selected] }
func (s *Select) SetError(err string)                        { s.error = err }
func (s *Select) SetFocused(focused bool)                    { s.focused = focused }
func (s *Select) SetDescription(fn func(string, int) string) { s.description = fn }

func (s *Select) Update(msg tea.Msg) (Prompt, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if s.selected > 0 {
				s.selected--
			}
		case "down", "j":
			if s.selected < len(s.options)-1 {
				s.selected++
			}
		}
	}
	return s, nil
}

func (s *Select) Render() string {
	var b strings.Builder

	// Question message
	style := blurredStyle
	if s.focused {
		style = focusedStyle
	}
	b.WriteString(style.Render(s.message))

	if s.defaultValue != "" {
		b.WriteString(helpStyle.Render(fmt.Sprintf(" (%s)", s.defaultValue)))
	}
	b.WriteString("\n")

	// Options
	for i, option := range s.options {
		prefix := "  "
		if i == s.selected {
			if s.focused {
				prefix = focusedStyle.Render("> ")
			} else {
				prefix = "> "
			}
		}

		optionText := option
		if s.description != nil {
			if desc := s.description(option, i); desc != "" {
				optionText += helpStyle.Render(" - " + desc)
			}
		}

		if i == s.selected && s.focused {
			b.WriteString(focusedStyle.Render(prefix + optionText))
		} else {
			b.WriteString(prefix + optionText)
		}
		b.WriteString("\n")
	}

	// Error message
	if s.error != "" {
		b.WriteString(errorStyle.Render("✗ " + s.error))
		b.WriteString("\n")
	}

	return b.String()
}
