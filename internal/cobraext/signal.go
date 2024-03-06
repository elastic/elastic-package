// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cobraext

import (
	"os"

	"github.com/spf13/cobra"
)

const signalHandlingAnnotation = "enable_signal_handling"

func EnableSignalHandling(cmd *cobra.Command) {
	cmd.Annotations[signalHandlingAnnotation] = ""
}

func IsSignalHandingRequested(cmd *cobra.Command) bool {
	_, found := getCommandAnnotation(cmd, signalHandlingAnnotation)
	return found
}

func getCommandAnnotation(cmd *cobra.Command, key string) (string, bool) {
	if len(os.Args) == 0 {
		return "", false
	}
	cmd, _, err := cmd.Root().Find(os.Args[1:])
	if err != nil {
		return "", false
	}
	for ; cmd.HasParent(); cmd = cmd.Parent() {
		if value, found := cmd.Annotations[key]; found {
			return value, true
		}
	}
	return "", false
}
