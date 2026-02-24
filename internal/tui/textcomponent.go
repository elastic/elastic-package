// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package tui

import (
	"errors"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// TextComponentMode determines if the component is read-only or editable
type TextComponentMode int

const (
	ViewMode TextComponentMode = iota
	EditMode
)

// TextComponent represents a unified text display/input component that can be read-only or editable
type TextComponent struct {
	title        string
	content      string
	mode         TextComponentMode
	message      string
	defaultValue string

	// View mode fields
	lines    []string
	viewport int
	offset   int
	hoffset  int // horizontal offset for wide content
	width    int
	height   int
	maxLines int
	maxWidth int

	// Edit mode fields
	textarea textarea.Model
	focused  bool
	error    string

	// Common fields
	submitted bool
	cancelled bool
	finished  bool
}

// TextComponentOptions holds optional parameters for creating a TextComponent
type TextComponentOptions struct {
	Mode         TextComponentMode
	Title        string
	Content      string
	Message      string
	DefaultValue string
	Focused      bool
}

// NewTextComponent creates a new text component with the given options
func NewTextComponent(opts TextComponentOptions) *TextComponent {
	tc := &TextComponent{
		title:        opts.Title,
		content:      opts.Content,
		mode:         opts.Mode,
		message:      opts.Message,
		defaultValue: opts.DefaultValue,
		focused:      opts.Focused,
		width:        80,
		height:       24,
	}

	// If content is empty but defaultValue is set, use defaultValue as content
	if tc.content == "" && tc.defaultValue != "" {
		tc.content = tc.defaultValue
	}

	if tc.mode == ViewMode {
		tc.initViewMode()
	} else {
		tc.initEditMode()
	}

	return tc
}

// ShowContent displays content in a scrollable viewer and waits for user to close it
func ShowContent(title, content string) error {
	component := NewTextComponent(TextComponentOptions{
		Mode:    ViewMode,
		Title:   title,
		Content: content,
	})
	model := newTextComponentModel(component)

	// Enable mouse support and alternate screen for better display
	program := tea.NewProgram(model, tea.WithAltScreen(), tea.WithMouseCellMotion())

	_, err := program.Run()
	if err != nil {
		return err
	}

	return nil
}

// AskTextArea runs a text area dialog for multi-line input
func AskTextArea(message string) (string, error) {
	component := NewTextComponent(TextComponentOptions{
		Mode:    EditMode,
		Message: message,
		Focused: true,
	})
	model := newTextComponentModel(component)
	program := tea.NewProgram(model)

	finalModel, err := program.Run()
	if err != nil {
		return "", err
	}

	result := finalModel.(*textComponentModel).component
	if result.cancelled {
		return "", ErrCancelled
	}

	if result.submitted {
		return strings.TrimSpace(result.textarea.Value()), nil
	}

	return "", ErrCancelled
}

// ErrCancelled is returned when user cancels the dialog
var ErrCancelled = errors.New("cancelled by user")

func (tc *TextComponent) initViewMode() {
	tc.lines = strings.Split(tc.content, "\n")
	tc.maxLines = len(tc.lines)
	tc.viewport = 18 // Leave space for header and footer

	// Calculate maximum line width for horizontal scrolling
	tc.maxWidth = 0
	for _, line := range tc.lines {
		if len(line) > tc.maxWidth {
			tc.maxWidth = len(line)
		}
	}
}

func (tc *TextComponent) initEditMode() {
	ta := textarea.New()
	ta.Placeholder = "Enter your text here... (ESC to cancel, Ctrl+D to submit)"
	ta.SetWidth(80)
	ta.SetHeight(16)
	ta.Focus()
	ta.SetValue(tc.content)

	// Custom key bindings - disable the default submit on enter
	ta.KeyMap.InsertNewline.SetEnabled(true)

	tc.textarea = ta
}

// textComponentModel is the bubbletea model for the unified text component
type textComponentModel struct {
	component *TextComponent
}

// newTextComponentModel creates a new model for the text component
func newTextComponentModel(component *TextComponent) *textComponentModel {
	return &textComponentModel{component: component}
}

func (m *textComponentModel) Init() tea.Cmd {
	if m.component.mode == EditMode {
		return textarea.Blink
	}
	return tea.EnterAltScreen
}

func (m *textComponentModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.component.width = msg.Width
		m.component.height = msg.Height
		if m.component.mode == ViewMode {
			// Leave more space for header, content borders, footer, and instructions
			m.component.viewport = msg.Height - 8
			if m.component.viewport < 1 {
				m.component.viewport = 1
			}
		}
		return m, nil

	case tea.KeyMsg:
		if m.component.mode == ViewMode {
			return m.updateViewMode(msg)
		} else {
			// Handle special keys first
			cancelled, submitted := handleEditModeKeys(msg.String())
			if cancelled {
				m.component.cancelled = true
				return m, tea.Quit
			}
			if submitted {
				m.component.submitted = true
				return m, tea.Quit
			}

			// For regular keys, update the textarea
			var cmd tea.Cmd
			m.component.textarea, cmd = m.component.textarea.Update(msg)
			return m, cmd
		}
	}

	// For edit mode, update the textarea for non-key events (i.e. Blink)
	if m.component.mode == EditMode {
		var cmd tea.Cmd
		m.component.textarea, cmd = m.component.textarea.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m *textComponentModel) updateViewMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc", "enter":
		m.component.finished = true
		return m, tea.Quit

	// Single line navigation
	case "up", "k":
		if m.component.offset > 0 {
			m.component.offset--
		}

	case "down", "j":
		maxOffset := m.component.maxLines - m.component.viewport
		if maxOffset < 0 {
			maxOffset = 0
		}
		if m.component.offset < maxOffset {
			m.component.offset++
		}

	// Horizontal navigation
	case "left", "h":
		if m.component.hoffset > 0 {
			m.component.hoffset--
		}

	case "right", "l":
		contentWidth := m.component.width - 8 // Account for border and padding
		maxHOffset := m.component.maxWidth - contentWidth
		if maxHOffset < 0 {
			maxHOffset = 0
		}
		if m.component.hoffset < maxHOffset {
			m.component.hoffset++
		}

	// Full page navigation
	case "pgup", "ctrl+b", "b":
		m.component.offset -= m.component.viewport
		if m.component.offset < 0 {
			m.component.offset = 0
		}

	case "pgdown", "ctrl+f", "f", " ":
		maxOffset := m.component.maxLines - m.component.viewport
		if maxOffset < 0 {
			maxOffset = 0
		}
		m.component.offset += m.component.viewport
		if m.component.offset > maxOffset {
			m.component.offset = maxOffset
		}

	// Top/bottom navigation
	case "home", "g":
		m.component.offset = 0

	case "end", "G":
		maxOffset := m.component.maxLines - m.component.viewport
		if maxOffset < 0 {
			maxOffset = 0
		}
		m.component.offset = maxOffset
	}

	return m, nil
}

// handleEditModeKeys handles common key events for edit mode components
// Returns (cancelled, submitted) flags
func handleEditModeKeys(key string) (cancelled bool, submitted bool) {
	switch key {
	case "esc", "ctrl+c":
		return true, false
	case "ctrl+d":
		return false, true
	}
	return false, false
}

func (m *textComponentModel) View() string {
	if m.component.mode == ViewMode {
		return m.viewModeRender()
	} else {
		return m.editModeRender()
	}
}

func (m *textComponentModel) viewModeRender() string {
	var b strings.Builder

	// Header with title and scroll position
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(ansiBrightWhite).
		Background(ansiBlue).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(ansiBrightBlue).
		BorderBottom(true).
		Width(m.component.width).
		MarginBottom(1). // Add space after header
		Padding(0, 2).   // Add horizontal padding
		Align(lipgloss.Center)

	scrollInfo := ""
	if m.component.maxLines > m.component.viewport {
		lineStart := m.component.offset + 1
		lineEnd := m.component.offset + m.component.viewport
		if lineEnd > m.component.maxLines {
			lineEnd = m.component.maxLines
		}
		scrollInfo = fmt.Sprintf(" | Lines %d-%d of %d", lineStart, lineEnd, m.component.maxLines)
	}

	// Add horizontal position if content is wider than viewport
	contentWidth := m.component.width - 8
	if m.component.maxWidth > contentWidth {
		hPos := m.component.hoffset + 1
		scrollInfo += fmt.Sprintf(" | Col %d", hPos)
	}

	titleText := m.component.title
	if scrollInfo != "" {
		titleText = fmt.Sprintf("%s%s", m.component.title, scrollInfo)
	}

	// Ensure title is not empty
	if titleText == "" {
		titleText = "Content Viewer"
	}

	b.WriteString(headerStyle.Render(titleText))
	b.WriteString("\n")

	// Content area
	contentStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(ansiBlue).
		Padding(1).
		Width(m.component.width - 4)

	var contentLines []string
	end := m.component.offset + m.component.viewport
	if end > m.component.maxLines {
		end = m.component.maxLines
	}
	for i := m.component.offset; i < end; i++ {
		line := m.component.lines[i]

		// Apply horizontal scrolling
		if m.component.hoffset > 0 && len(line) > m.component.hoffset {
			line = line[m.component.hoffset:]
		} else if m.component.hoffset > 0 {
			line = ""
		}

		// Truncate line if it's too wide
		if len(line) > contentWidth {
			line = line[:contentWidth]
		}

		contentLines = append(contentLines, line)
	}

	// Pad with empty lines if needed
	for len(contentLines) < m.component.viewport {
		contentLines = append(contentLines, "")
	}

	content := strings.Join(contentLines, "\n")
	b.WriteString(contentStyle.Render(content))

	// Footer instructions
	b.WriteString("\n")
	instructionsStyle := lipgloss.NewStyle().
		Foreground(ansiBrightBlack).
		Italic(true)

	instructions := "↑↓/jk: line | ←→/hl: scroll | PgUp/PgDn/Ctrl+B/Ctrl+F/b/f/Space: page | Home/End/g/G: top/bottom | Enter/q/Esc: close"
	b.WriteString(instructionsStyle.Render(instructions))

	return b.String()
}

// renderEditMode renders the edit mode UI for a text component
func renderEditMode(message string, focused bool, textarea textarea.Model, error string) string {
	var b strings.Builder

	// Question message
	style := blurredStyle
	if focused {
		style = focusedStyle
	}
	b.WriteString(style.Render(message))
	b.WriteString("\n")

	// Instructions
	if focused {
		b.WriteString(helpStyle.Render("  Use Ctrl+D to submit, ESC to cancel"))
		b.WriteString("\n\n")
	}

	// TextArea
	b.WriteString(textarea.View())

	// Error message
	if error != "" {
		b.WriteString("\n")
		b.WriteString(errorStyle.Render("✗ " + error))
	}

	return b.String()
}

func (m *textComponentModel) editModeRender() string {
	return renderEditMode(m.component.message, m.component.focused, m.component.textarea, m.component.error)
}
