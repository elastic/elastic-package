package cobraext

import (
	"github.com/spf13/cobra"
)

// CommandAction defines the signature of a cobra command action function
type CommandAction func(cmd *cobra.Command, args []string) error

// ComposeCommandActions runs the given command actions in order
func ComposeCommandActions(cmd *cobra.Command, args []string, actions ...CommandAction) error {
	for _, action := range actions {
		err := action(cmd, args)
		if err != nil {
			return err
		}
	}
	return nil
}
