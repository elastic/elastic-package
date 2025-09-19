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

// confirmItem implements list.Item for the confirm component
type confirmItem struct {
	title string
	value bool
}

func (i confirmItem) FilterValue() string { return i.title }
func (i confirmItem) Title() string       { return i.title }
func (i confirmItem) Description() string { return "" }

// Custom delegate with explicit selection indicator
type confirmDelegate struct{}

func (d confirmDelegate) Height() int                             { return 1 }
func (d confirmDelegate) Spacing() int                            { return 0 }
func (d confirmDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }
func (d confirmDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	i, ok := listItem.(confirmItem)
	if !ok {
		return
	}

	str := i.title

	// Show selection indicator
	if index == m.Index() {
		str = focusedStyle.Render("> " + str)
	} else {
		str = blurredStyle.Render("  " + str)
	}

	fmt.Fprint(w, str)
}

// Confirm represents a yes/no confirmation prompt using bubbles list
type Confirm struct {
	message      string
	defaultValue bool
	list         list.Model
	focused      bool
	error        string
}

// NewConfirm creates a new confirm prompt
func NewConfirm(message string, defaultValue bool) *Confirm {
	items := []list.Item{
		confirmItem{title: "Yes", value: true},
		confirmItem{title: "No", value: false},
	}

	selectedIndex := 1 // Default to "No" (index 1)
	if defaultValue {
		selectedIndex = 0 // "Yes" (index 0)
	}

	l := list.New(items, confirmDelegate{}, 20, 3) // Slightly larger height to ensure both options show
	l.SetShowStatusBar(false)
	l.SetShowTitle(false)
	l.SetShowHelp(false)
	l.SetShowPagination(false) // Disable pagination to show all options
	l.SetFilteringEnabled(false)
	l.Select(selectedIndex)

	// Custom styles - using existing styles from models.go
	l.Styles.PaginationStyle = helpStyle
	l.Styles.HelpStyle = helpStyle

	return &Confirm{
		message:      message,
		defaultValue: defaultValue,
		list:         l,
		focused:      true,
	}
}

func (c *Confirm) Message() string         { return c.message }
func (c *Confirm) Default() interface{}    { return c.defaultValue }
func (c *Confirm) SetError(err string)     { c.error = err }
func (c *Confirm) SetFocused(focused bool) { c.focused = focused }

func (c *Confirm) Value() interface{} {
	if item, ok := c.list.SelectedItem().(confirmItem); ok {
		return item.value
	}
	return c.defaultValue
}

func (c *Confirm) Update(msg tea.Msg) (Prompt, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch strings.ToLower(msg.String()) {
		case "y":
			c.list.Select(0) // Yes
			return c, nil
		case "n":
			c.list.Select(1) // No
			return c, nil
		}
	}

	var cmd tea.Cmd
	c.list, cmd = c.list.Update(msg)
	return c, cmd
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

	// List options
	b.WriteString(c.list.View())

	// Error message
	if c.error != "" {
		b.WriteString("\n")
		b.WriteString(errorStyle.Render("✗ " + c.error))
	}

	return b.String()
}
