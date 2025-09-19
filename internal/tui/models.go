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
	ansiBlack         = "0"
	ansiRed           = "1"
	ansiGreen         = "2"
	ansiYellow        = "3"
	ansiBlue          = "4"
	ansiMagenta       = "5"
	ansiCyan          = "6"
	ansiWhite         = "7"
	ansiBrightBlack   = "8" // Gray
	ansiBrightRed     = "9"
	ansiBrightGreen   = "10"
	ansiBrightYellow  = "11"
	ansiBrightBlue    = "12"
	ansiBrightMagenta = "13"
	ansiBrightCyan    = "14"
	ansiBrightWhite   = "15"
)

// colorSupported checks if color output is supported based on environment variables
func colorSupported() bool {
	// Check NO_COLOR environment variable (https://no-color.org/)
	if os.Getenv("NO_COLOR") != "" {
		return false
	}

	// Check TERM environment variable
	term := os.Getenv("TERM")
	switch {
	case term == "":
		return false
	case term == "dumb":
		return false
	default:
		// Default to supporting color for most modern terminals
		return true
	}
}

// getColor returns the color if colors are supported, empty string otherwise
func getColor(ansiColor string) lipgloss.Color {
	if !colorSupported() {
		return lipgloss.Color("")
	}
	return lipgloss.Color(ansiColor)
}

// Styles for consistent UI using ANSI 16 colors with NO_COLOR support
var (
	focusedStyle    lipgloss.Style
	blurredStyle    lipgloss.Style
	errorStyle      lipgloss.Style
	helpStyle       lipgloss.Style
	selectedStyle   lipgloss.Style
	unselectedStyle lipgloss.Style
)

// Initialize styles based on color support
func init() {
	if colorSupported() {
		// Color mode: use colors
		focusedStyle = lipgloss.NewStyle().Foreground(getColor(ansiBrightMagenta)).Bold(true)
		blurredStyle = lipgloss.NewStyle().Foreground(getColor(ansiBrightBlack))
		errorStyle = lipgloss.NewStyle().Foreground(getColor(ansiBrightRed))
		helpStyle = lipgloss.NewStyle().Foreground(getColor(ansiBrightBlack))
		selectedStyle = lipgloss.NewStyle().Foreground(getColor(ansiBrightGreen)).Bold(true)
		unselectedStyle = lipgloss.NewStyle().Foreground(getColor(ansiBrightBlack))
	} else {
		// NO_COLOR mode: use text formatting only
		focusedStyle = lipgloss.NewStyle().Bold(true)
		blurredStyle = lipgloss.NewStyle()
		errorStyle = lipgloss.NewStyle().Bold(true)
		helpStyle = lipgloss.NewStyle()
		selectedStyle = lipgloss.NewStyle().Bold(true)
		unselectedStyle = lipgloss.NewStyle()
	}
}

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

// Compile-time interface checks to ensure all prompt types implement the Prompt interface
var (
	_ Prompt = &Input{}
	_ Prompt = &Select{}
	_ Prompt = &Confirm{}
	_ Prompt = &MultiSelect{}
)
