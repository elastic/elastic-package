package cmd

import (
	"github.com/spf13/cobra"
)

// commandAction defines the signature of a cobra command action function
type commandAction func(cmd *cobra.Command, args []string) error

// composeCommandActions runs the given command actions in order
func composeCommandActions(cmd *cobra.Command, args []string, actions ...commandAction) error {
	for _, action := range actions {
		err := action(cmd, args)
		if err != nil {
			return err
		}
	}
	return nil
}
