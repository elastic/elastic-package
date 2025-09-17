// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package tui

import (
	"fmt"
	"reflect"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// questionnaireModel handles multiple questions
type questionnaireModel struct {
	questions       []*Question
	currentQuestion int
	answers         map[string]interface{}
	finished        bool
	err             error
	width           int
	height          int
}

func newQuestionnaireModel(questions []*Question) *questionnaireModel {
	return &questionnaireModel{
		questions: questions,
		answers:   make(map[string]interface{}),
		width:     80,
		height:    24,
	}
}

func (m *questionnaireModel) Init() tea.Cmd {
	return nil
}

func (m *questionnaireModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		if m.finished {
			return m, tea.Quit
		}

		switch msg.String() {
		case "ctrl+c":
			m.err = fmt.Errorf("cancelled by user")
			return m, tea.Quit

		case "enter":
			if err := m.validateCurrentAnswer(); err != nil {
				m.setCurrentError(err.Error())
				return m, nil
			}

			// Save current answer
			current := m.questions[m.currentQuestion]
			m.answers[current.Name] = current.Prompt.Value()

			// Move to next question or finish
			m.currentQuestion++
			if m.currentQuestion >= len(m.questions) {
				m.finished = true
				return m, tea.Quit
			}

			// Clear any previous error
			m.setCurrentError("")
			return m, nil
		}
	}

	// Update current prompt
	if !m.finished && m.currentQuestion < len(m.questions) {
		current := m.questions[m.currentQuestion]
		updatedPrompt, cmd := current.Prompt.Update(msg)
		current.Prompt = updatedPrompt
		return m, cmd
	}

	return m, nil
}

func (m *questionnaireModel) validateCurrentAnswer() error {
	current := m.questions[m.currentQuestion]
	if current.Validate != nil {
		return current.Validate(current.Prompt.Value())
	}
	return nil
}

func (m *questionnaireModel) setCurrentError(err string) {
	if m.currentQuestion < len(m.questions) {
		current := m.questions[m.currentQuestion]
		switch p := current.Prompt.(type) {
		case interface{ SetError(string) }:
			p.SetError(err)
		}
	}
}

func (m *questionnaireModel) View() string {
	if m.finished {
		return ""
	}

	var b strings.Builder

	// Header with progress
	progress := fmt.Sprintf("Question %d of %d", m.currentQuestion+1, len(m.questions))
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("86")).
		BorderStyle(lipgloss.NormalBorder()).
		BorderBottom(true).
		Width(m.width).
		Align(lipgloss.Center)

	b.WriteString(headerStyle.Render(progress))
	b.WriteString("\n\n")

	// Current question
	if m.currentQuestion < len(m.questions) {
		current := m.questions[m.currentQuestion]
		b.WriteString(current.Prompt.Render())
	}

	// Footer instructions
	b.WriteString("\n\n")
	instructions := "Press Enter to continue, Ctrl+C to cancel"
	if isMultiSelect(m.questions[m.currentQuestion].Prompt) {
		instructions = "Use ↑↓ to navigate, Space to select, Enter to continue, Ctrl+C to cancel"
	} else if isSelect(m.questions[m.currentQuestion].Prompt) {
		instructions = "Use ↑↓ to navigate, Enter to continue, Ctrl+C to cancel"
	}

	footerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Italic(true)
	b.WriteString(footerStyle.Render(instructions))

	return b.String()
}

func isMultiSelect(prompt Prompt) bool {
	_, ok := prompt.(*MultiSelect)
	return ok
}

func isSelect(prompt Prompt) bool {
	_, ok := prompt.(*Select)
	return ok
}

// Ask runs multiple questions and stores answers in the provided struct
func Ask(questions []*Question, answers interface{}) error {
	model := newQuestionnaireModel(questions)
	program := tea.NewProgram(model)

	finalModel, err := program.Run()
	if err != nil {
		return fmt.Errorf("failed to run questionnaire: %w", err)
	}

	result := finalModel.(*questionnaireModel)
	if result.err != nil {
		return result.err
	}

	// Map answers to struct
	return mapAnswersToStruct(result.answers, answers)
}

// AskOne runs a single question
func AskOne(prompt Prompt, answer interface{}, validators ...Validator) error {
	question := &Question{
		Name:   "answer",
		Prompt: prompt,
	}

	if len(validators) > 0 {
		question.Validate = ComposeValidators(validators...)
	}

	// Run the single question directly
	model := newQuestionnaireModel([]*Question{question})
	program := tea.NewProgram(model)

	finalModel, err := program.Run()
	if err != nil {
		return fmt.Errorf("failed to run prompt: %w", err)
	}

	result := finalModel.(*questionnaireModel)
	if result.err != nil {
		return result.err
	}

	// Extract the single answer directly
	if val, ok := result.answers["answer"]; ok {
		return assignValue(answer, val)
	}

	return fmt.Errorf("no answer received")
}

// mapAnswersToStruct maps the answers map to a struct using reflection
func mapAnswersToStruct(answers map[string]interface{}, target interface{}) error {
	v := reflect.ValueOf(target)
	if v.Kind() != reflect.Ptr {
		return fmt.Errorf("target must be a pointer to struct")
	}

	v = v.Elem()
	if v.Kind() != reflect.Struct {
		return fmt.Errorf("target must be a pointer to struct")
	}

	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		fieldType := t.Field(i)

		// Skip unexported fields
		if !field.CanSet() {
			continue
		}

		// Look for matching answer by survey tag first, then field name (case-insensitive)
		var answerKey string

		// Check if field has a survey tag
		if surveyTag := fieldType.Tag.Get("survey"); surveyTag != "" {
			for key := range answers {
				if strings.EqualFold(key, surveyTag) {
					answerKey = key
					break
				}
			}
		}

		// If no survey tag match, try field name
		if answerKey == "" {
			for key := range answers {
				if strings.EqualFold(key, fieldType.Name) {
					answerKey = key
					break
				}
			}
		}

		if answerKey == "" {
			continue
		}

		answer := answers[answerKey]
		if err := assignValue(field.Addr().Interface(), answer); err != nil {
			return fmt.Errorf("failed to assign value to field %s: %w", fieldType.Name, err)
		}
	}

	return nil
}

// assignValue assigns a value to a pointer target
func assignValue(target interface{}, value interface{}) error {
	targetValue := reflect.ValueOf(target)
	if targetValue.Kind() != reflect.Ptr {
		return fmt.Errorf("target must be a pointer")
	}

	targetValue = targetValue.Elem()
	sourceValue := reflect.ValueOf(value)

	if !sourceValue.Type().AssignableTo(targetValue.Type()) {
		// Try conversion for compatible types
		if sourceValue.Type().ConvertibleTo(targetValue.Type()) {
			sourceValue = sourceValue.Convert(targetValue.Type())
		} else {
			return fmt.Errorf("cannot assign %T to %T", value, target)
		}
	}

	targetValue.Set(sourceValue)
	return nil
}
