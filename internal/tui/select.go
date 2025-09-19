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
	"github.com/charmbracelet/lipgloss"
)

// selectItem implements list.Item for the select component
type selectItem struct {
	title       string
	description string
}

func (i selectItem) FilterValue() string { return i.title }
func (i selectItem) Title() string       { return i.title }
func (i selectItem) Description() string { return i.description }

// Custom item delegate for select
type selectDelegate struct{}

func (d selectDelegate) Height() int                             { return 1 }
func (d selectDelegate) Spacing() int                            { return 0 }
func (d selectDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }
func (d selectDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	i, ok := listItem.(selectItem)
	if !ok {
		return
	}

	str := fmt.Sprintf("%s", i.title)
	if i.description != "" {
		str += helpStyle.Render(" - " + i.description)
	}

	fn := blurredStyle.Render
	if index == m.Index() {
		fn = func(s ...string) string {
			return focusedStyle.Render("> " + strings.Join(s, " "))
		}
	} else {
		str = "  " + str
	}

	fmt.Fprint(w, fn(str))
}

// Select represents a single-choice selection prompt using bubbles list
type Select struct {
	message      string
	options      []string
	defaultValue string
	list         list.Model
	focused      bool
	error        string
	description  func(string, int) string
}

// NewSelect creates a new select prompt
func NewSelect(message string, options []string, defaultValue string) *Select {
	items := make([]list.Item, len(options))
	selectedIndex := 0

	for i, opt := range options {
		items[i] = selectItem{title: opt}
		if opt == defaultValue {
			selectedIndex = i
		}
	}

	l := list.New(items, selectDelegate{}, 50, len(options))
	l.SetShowStatusBar(false)
	l.SetShowTitle(false)
	l.SetShowHelp(false)
	l.SetFilteringEnabled(false)
	l.Select(selectedIndex)

	// Custom styles
	l.Styles.Title = lipgloss.NewStyle()
	l.Styles.PaginationStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	l.Styles.HelpStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))

	return &Select{
		message:      message,
		options:      options,
		defaultValue: defaultValue,
		list:         l,
		focused:      true,
	}
}

func (s *Select) Message() string         { return s.message }
func (s *Select) Default() interface{}    { return s.defaultValue }
func (s *Select) SetError(err string)     { s.error = err }
func (s *Select) SetFocused(focused bool) { s.focused = focused }

func (s *Select) Value() interface{} {
	if item, ok := s.list.SelectedItem().(selectItem); ok {
		return item.title
	}
	return s.defaultValue
}

func (s *Select) SetDescription(fn func(string, int) string) { 
	s.description = fn
	// Update items with descriptions
	items := make([]list.Item, len(s.options))
	for i, opt := range s.options {
		desc := ""
		if fn != nil {
			desc = fn(opt, i)
		}
		items[i] = selectItem{title: opt, description: desc}
	}
	s.list.SetItems(items)
}

func (s *Select) Update(msg tea.Msg) (Prompt, tea.Cmd) {
	var cmd tea.Cmd
	s.list, cmd = s.list.Update(msg)
	return s, cmd
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

	// List
	b.WriteString(s.list.View())

	// Error message
	if s.error != "" {
		b.WriteString("\n")
		b.WriteString(errorStyle.Render("✗ " + s.error))
	}

	return b.String()
}
