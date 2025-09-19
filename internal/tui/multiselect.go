// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package tui

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

// multiSelectItem implements list.Item for the multiselect component
type multiSelectItem struct {
	title       string
	description string
	selected    bool
	index       int
}

func (i multiSelectItem) FilterValue() string { return i.title }
func (i multiSelectItem) Title() string       { return i.title }
func (i multiSelectItem) Description() string { return i.description }

// Custom delegate for multiselect with checkbox functionality
type multiSelectDelegate struct{}

func newMultiSelectDelegate() multiSelectDelegate {
	return multiSelectDelegate{}
}

func (d multiSelectDelegate) Height() int                             { return 1 }
func (d multiSelectDelegate) Spacing() int                            { return 0 }
func (d multiSelectDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }
func (d multiSelectDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	i, ok := listItem.(multiSelectItem)
	if !ok {
		return
	}

	checkbox := unselectedStyle.Render("[ ]")
	if i.selected {
		checkbox = selectedStyle.Render("[✓]")
	}

	title := i.title
	if i.description != "" {
		title += helpStyle.Render(" - " + i.description)
	}

	content := checkbox + " " + title
	if index == m.Index() {
		content = focusedStyle.Render("> " + content)
	} else {
		content = "  " + content
	}

	fmt.Fprint(w, content)
}

// MultiSelect represents a multiple-choice selection prompt using bubbles list
type MultiSelect struct {
	message      string
	options      []string
	defaultValue []string
	selected     map[int]bool
	list         list.Model
	focused      bool
	error        string
	description  func(string, int) string
}

// NewMultiSelect creates a new multi-select prompt
func NewMultiSelect(message string, options []string, defaultValue []string) *MultiSelect {
	ms := &MultiSelect{
		message:      message,
		options:      options,
		defaultValue: defaultValue,
		selected:     make(map[int]bool),
		focused:      true,
	}

	items := make([]list.Item, len(options))
	for i, opt := range options {
		// Check if this option is in the default values
		isSelected := false
		for _, defaultVal := range defaultValue {
			if opt == defaultVal {
				isSelected = true
				ms.selected[i] = true
				break
			}
		}
		items[i] = multiSelectItem{
			title:    opt,
			selected: isSelected,
			index:    i,
		}
	}

	delegate := newMultiSelectDelegate()
	// Calculate height: exact number of items needed, max 20 for scrolling
	listHeight := min(len(options), 20)
	l := list.New(items, delegate, 80, listHeight)
	l.SetShowStatusBar(false)
	l.SetShowTitle(false)
	l.SetShowHelp(false)
	l.SetShowPagination(false) // Disable pagination, use scrolling instead
	l.SetFilteringEnabled(false)

	// Custom styles - using existing styles from models.go
	l.Styles.PaginationStyle = helpStyle
	l.Styles.HelpStyle = helpStyle

	ms.list = l
	return ms
}

func (m *MultiSelect) Message() string         { return m.message }
func (m *MultiSelect) Default() interface{}    { return m.defaultValue }
func (m *MultiSelect) SetError(err string)     { m.error = err }
func (m *MultiSelect) SetFocused(focused bool) { m.focused = focused }
func (m *MultiSelect) SetPageSize(size int) {
	// Ensure the size doesn't exceed our max of 20 and isn't larger than options
	adjustedSize := min(size, min(len(m.options), 20))
	m.list.SetHeight(adjustedSize)
}

func (m *MultiSelect) SetDescription(fn func(string, int) string) {
	m.description = fn
	// Update items with descriptions
	items := make([]list.Item, len(m.options))
	for i, opt := range m.options {
		desc := ""
		if fn != nil {
			desc = fn(opt, i)
		}
		items[i] = multiSelectItem{
			title:       opt,
			description: desc,
			selected:    m.selected[i],
			index:       i,
		}
	}
	m.list.SetItems(items)
	// Recalculate height after setting items
	listHeight := min(len(m.options), 20)
	m.list.SetHeight(listHeight)
}

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
		case " ":
			// Toggle selection for current item
			currentIndex := m.list.Index()
			m.selected[currentIndex] = !m.selected[currentIndex]

			// Update the item to reflect the new selection state
			if item, ok := m.list.SelectedItem().(multiSelectItem); ok {
				item.selected = m.selected[currentIndex]
				items := m.list.Items()
				items[currentIndex] = item
				m.list.SetItems(items)
			}
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
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

	// List
	b.WriteString(m.list.View())

	// Error message
	if m.error != "" {
		b.WriteString("\n")
		b.WriteString(errorStyle.Render("✗ " + m.error))
	}

	return b.String()
}

// min helper function for Go versions that don't have it built-in
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
