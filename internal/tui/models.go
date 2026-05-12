// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package tui

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Masterminds/semver/v3"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/elastic/elastic-package/internal/install"
)

// Question represents a single prompt question
type Question struct {
	Name     string
	Prompt   Prompt
	Validate ValidatorFunc
}

// Prompt interface for different prompt types
type Prompt interface {
	Render() string
	Update(msg tea.Msg) (Prompt, tea.Cmd)
	Value() interface{}
	Message() string
	Default() interface{}
}

// ValidatorFunc function type for validation
type ValidatorFunc func(interface{}) error

// ANSI 16 color constants
const (
	ansiBlack         = lipgloss.Color("0")
	ansiRed           = lipgloss.Color("1")
	ansiGreen         = lipgloss.Color("2")
	ansiYellow        = lipgloss.Color("3")
	ansiBlue          = lipgloss.Color("4")
	ansiMagenta       = lipgloss.Color("5")
	ansiCyan          = lipgloss.Color("6")
	ansiWhite         = lipgloss.Color("7")
	ansiBrightBlack   = lipgloss.Color("8") // Gray
	ansiBrightRed     = lipgloss.Color("9")
	ansiBrightGreen   = lipgloss.Color("10")
	ansiBrightYellow  = lipgloss.Color("11")
	ansiBrightBlue    = lipgloss.Color("12")
	ansiBrightMagenta = lipgloss.Color("13")
	ansiBrightCyan    = lipgloss.Color("14")
	ansiBrightWhite   = lipgloss.Color("15")
)

var (
	focusedStyle    = lipgloss.NewStyle().Foreground(ansiBrightMagenta).Bold(true)
	blurredStyle    = lipgloss.NewStyle().Foreground(ansiBrightBlack)
	errorStyle      = lipgloss.NewStyle().Foreground(ansiBrightRed)
	helpStyle       = lipgloss.NewStyle().Foreground(ansiBrightBlack)
	selectedStyle   = lipgloss.NewStyle().Foreground(ansiBrightGreen).Bold(true)
	unselectedStyle = lipgloss.NewStyle().Foreground(ansiBrightBlack)

	// Console output styles
	warningStyle = lipgloss.NewStyle().Foreground(ansiYellow)
	infoStyle    = lipgloss.NewStyle().Foreground(ansiCyan)
	successStyle = lipgloss.NewStyle().Foreground(ansiGreen).Bold(true)
)

// ComposeValidators combines multiple validators
func ComposeValidators(validators ...ValidatorFunc) ValidatorFunc {
	return func(val interface{}) error {
		for _, validator := range validators {
			if err := validator(val); err != nil {
				return err
			}
		}
		return nil
	}
}

// Required validator
func Required(val interface{}) error {
	switch v := val.(type) {
	case string:
		if strings.TrimSpace(v) == "" {
			return fmt.Errorf("this field is required")
		}
	case []string:
		if len(v) == 0 {
			return fmt.Errorf("at least one option must be selected")
		}
	}
	return nil
}

// Validation patterns
var (
	githubOwnerRegexp = regexp.MustCompile(`^(([a-zA-Z0-9-_]+)|([a-zA-Z0-9-_]+\/[a-zA-Z0-9-_]+))$`)

	packageNameRegexp    = regexp.MustCompile(`^[a-z0-9_]+$`)
	dataStreamNameRegexp = regexp.MustCompile(`^([a-z0-9]{2}|[a-z0-9][a-z0-9_]+[a-z0-9])$`)
)

// Validator struct for package and data stream validation
type Validator struct {
	Cwd string
}

// PackageDoesNotExist function checks if the package hasn't been already created.
func (v Validator) PackageDoesNotExist(val interface{}) error {
	baseDir, ok := val.(string)
	if !ok {
		return errors.New("string type expected")
	}
	_, err := os.Stat(filepath.Join(v.Cwd, baseDir))
	if err == nil {
		return fmt.Errorf(`package "%s" already exists`, baseDir)
	}
	return nil
}

// DataStreamDoesNotExist function checks if the package doesn't contain the data stream.
func (v Validator) DataStreamDoesNotExist(val interface{}) error {
	name, ok := val.(string)
	if !ok {
		return errors.New("string type expected")
	}

	dataStreamDir := filepath.Join(v.Cwd, "data_stream", name)
	_, err := os.Stat(dataStreamDir)
	if err == nil {
		return fmt.Errorf(`data stream "%s" already exists`, name)
	}
	return nil
}

// Semver function checks if the value is a correct semver.
func (v Validator) Semver(val interface{}) error {
	ver, ok := val.(string)
	if !ok {
		return errors.New("string type expected")
	}
	_, err := semver.NewVersion(ver)
	if err != nil {
		return fmt.Errorf("can't parse value as proper semver: %w", err)
	}
	return nil
}

// Constraint function checks if the value is a correct version constraint.
func (v Validator) Constraint(val interface{}) error {
	c, ok := val.(string)
	if !ok {
		return errors.New("string type expected")
	}
	_, err := semver.NewConstraint(c)
	if err != nil {
		return fmt.Errorf("can't parse value as proper constraint: %w", err)
	}
	return nil
}

// GithubOwner function checks if the Github owner is valid (team or user)
func (v Validator) GithubOwner(val interface{}) error {
	githubOwner, ok := val.(string)
	if !ok {
		return errors.New("string type expected")
	}

	if !githubOwnerRegexp.MatchString(githubOwner) {
		return fmt.Errorf("value doesn't match the regular expression (organization/group or username): %s", githubOwnerRegexp.String())
	}
	return nil
}

// PackageName validates package names
func (v Validator) PackageName(val interface{}) error {
	packageName, ok := val.(string)
	if !ok {
		return errors.New("string type expected")
	}

	if !packageNameRegexp.MatchString(packageName) {
		return fmt.Errorf("value doesn't match the regular expression (package name): %s", packageNameRegexp.String())
	}
	return nil
}

// DataStreamName validates data stream names
func (v Validator) DataStreamName(val interface{}) error {
	dataStreamFolderName, ok := val.(string)
	if !ok {
		return errors.New("string type expected")
	}

	if !dataStreamNameRegexp.MatchString(dataStreamFolderName) {
		return fmt.Errorf("value doesn't match the regular expression (datastream name): %s", dataStreamNameRegexp.String())
	}
	return nil
}

// DefaultKibanaVersionConditionValue function returns a constraint
func DefaultKibanaVersionConditionValue() string {
	ver := semver.MustParse(install.DefaultStackVersion)
	v, _ := ver.SetPrerelease("")
	return "^" + v.String()
}

// Warning renders text in warning color
func Warning(text string) string {
	return warningStyle.Render(text)
}

// Info renders text in info color
func Info(text string) string {
	return infoStyle.Render(text)
}

// Success renders text in success color
func Success(text string) string {
	return successStyle.Render(text)
}

// Error renders text in error color
func Error(text string) string {
	return errorStyle.Render(text)
}
