package main

import (
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

func setupCheckCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "check",
		Short: "Check the integration",
		Long:  "Use check command to verify if the integration is correct in terms of formatting, validation, building, tests.",
		RunE: func(cmd *cobra.Command, args []string) error {
			err := composeCommandActions(cmd, args,
				formatCommandAction,
				validateCommandAction,
				buildCommandAction,
				testCommandAction,
			)
			if err != nil {
				return errors.Wrap(err, "checking integration failed")
			}
			return nil
		},
	}
	return cmd
}

func composeCommandActions(cmd *cobra.Command, args []string, actions ...func(cmd *cobra.Command, args []string) error) error {
	for _, action := range actions {
		err := action(cmd, args)
		if err != nil {
			return err
		}
	}
	return nil
}
