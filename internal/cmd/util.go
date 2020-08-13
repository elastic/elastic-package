package cmd

import (
	"github.com/spf13/cobra"
)

func ComposeCommandActions(cmd *cobra.Command, args []string, actions ...func(cmd *cobra.Command, args []string) error) error {
	for _, action := range actions {
		err := action(cmd, args)
		if err != nil {
			return err
		}
	}
	return nil
}
