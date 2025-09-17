// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// MultiSelect represents a multiple-choice selection prompt
type MultiSelect struct {
	message      string
	options      []string
	defaultValue []string
	selected     []bool
	cursor       int
	focused      bool
	error        string
	description  func(string, int) string
	pageSize     int
	scrollOffset int
}

// NewMultiSelect creates a new multi-select prompt
func NewMultiSelect(message string, options []string, defaultValue []string) *MultiSelect {
	selected := make([]bool, len(options))

	// Mark default values as selected
	for _, defaultVal := range defaultValue {
		for i, opt := range options {
			if opt == defaultVal {
				selected[i] = true
				break
			}
		}
	}

	return &MultiSelect{
		message:      message,
		options:      options,
		defaultValue: defaultValue,
		selected:     selected,
		focused:      true,
		pageSize:     10, // Default page size
	}
}

func (m *MultiSelect) Message() string                            { return m.message }
func (m *MultiSelect) Default() interface{}                       { return m.defaultValue }
func (m *MultiSelect) SetError(err string)                        { m.error = err }
func (m *MultiSelect) SetFocused(focused bool)                    { m.focused = focused }
func (m *MultiSelect) SetDescription(fn func(string, int) string) { m.description = fn }
func (m *MultiSelect) SetPageSize(size int)                       { m.pageSize = size }

func (m *MultiSelect) Value() interface{} {
	var result []string
	for i, isSelected := range m.selected {
		if isSelected {
			result = append(result, m.options[i])
		}
	}
	return result
}

func (m *MultiSelect) Update(msg tea.Msg) (Prompt, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
				m.updateScroll()
			}
		case "down", "j":
			if m.cursor < len(m.options)-1 {
				m.cursor++
				m.updateScroll()
			}
		case " ":
			// Toggle selection
			m.selected[m.cursor] = !m.selected[m.cursor]
		}
	}
	return m, nil
}

func (m *MultiSelect) updateScroll() {
	if m.cursor < m.scrollOffset {
		m.scrollOffset = m.cursor
	} else if m.cursor >= m.scrollOffset+m.pageSize {
		m.scrollOffset = m.cursor - m.pageSize + 1
	}
}

func (m *MultiSelect) Render() string {
	var b strings.Builder

	// Question message
	style := blurredStyle
	if m.focused {
		style = focusedStyle
	}
	b.WriteString(style.Render(m.message))

	if len(m.defaultValue) > 0 {
		b.WriteString(helpStyle.Render(fmt.Sprintf(" (%s)", strings.Join(m.defaultValue, ", "))))
	}
	b.WriteString("\n")

	// Show instructions
	if m.focused {
		b.WriteString(helpStyle.Render("  Use ↑↓ to navigate, space to toggle selection, enter to confirm"))
		b.WriteString("\n")
	}

	// Calculate visible range
	start := m.scrollOffset
	end := start + m.pageSize
	if end > len(m.options) {
		end = len(m.options)
	}

	// Show scroll indicator if needed
	if start > 0 {
		b.WriteString(helpStyle.Render("  ↑ more options above"))
		b.WriteString("\n")
	}

	// Options
	for i := start; i < end; i++ {
		prefix := "  "
		var checkbox, optionText string

		// Better visual indicators for selection
		if m.selected[i] {
			checkbox = selectedStyle.Render("[✓]")
			optionText = selectedStyle.Render(m.options[i])
		} else {
			checkbox = unselectedStyle.Render("[ ]")
			optionText = m.options[i]
		}

		// Handle cursor highlighting
		if i == m.cursor {
			if m.focused {
				prefix = focusedStyle.Render("> ")
				// Make the cursor line more prominent
				if m.selected[i] {
					checkbox = focusedStyle.Render("[✓]")
					optionText = focusedStyle.Render(m.options[i])
				} else {
					checkbox = focusedStyle.Render("[ ]")
					optionText = focusedStyle.Render(m.options[i])
				}
			} else {
				prefix = "> "
			}
		}

		// Add description if available
		if m.description != nil {
			if desc := m.description(m.options[i], i); desc != "" {
				optionText += helpStyle.Render(" - " + desc)
			}
		}

		line := prefix + checkbox + " " + optionText
		b.WriteString(line)
		b.WriteString("\n")
	}

	// Show scroll indicator if needed
	if end < len(m.options) {
		b.WriteString(helpStyle.Render("  ↓ more options below"))
		b.WriteString("\n")
	}

	// Error message
	if m.error != "" {
		b.WriteString(errorStyle.Render("✗ " + m.error))
		b.WriteString("\n")
	}

	return b.String()
}
