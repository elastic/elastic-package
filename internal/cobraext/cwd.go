// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cobraext

import (
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

func Getwd(cmd *cobra.Command) (string, error) {
	cwd, err := cmd.Flags().GetString(ChangeDirectoryFlagName)
	if err != nil {
		return "", FlagParsingError(err, ChangeDirectoryFlagName)
	}
	if cwd == "" {
		return os.Getwd() //permit:os.Getwd // This should be the only place where this is needed.
	}
	abs, err := filepath.Abs(cwd) //permit:filepath.Abs // This should be the only place where this is needed.
	if err != nil {
		return "", FlagParsingError(err, ChangeDirectoryFlagName)
	}
	return abs, nil
}

// AbsolutePathFlag obtains a flag with a path as an absolute path, before changing any directory.
func AbsolutePathFlag(cmd *cobra.Command, name string) (string, error) {
	pathFlag, err := cmd.Flags().GetString(name)
	if err != nil {
		return "", FlagParsingError(err, name)
	}
	if pathFlag != "" {
		abs, err := filepath.Abs(pathFlag) //permit:filepath.Abs // This should be the only place where this is needed.
		if err != nil {
			return "", FlagParsingError(err, ChangeDirectoryFlagName)
		}
		pathFlag = abs
	}
	return pathFlag, nil
}
