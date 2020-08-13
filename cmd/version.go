package cmd

import (
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"strconv"
	"time"
)

var (
	// BuildTime is the build time of the binary (set externally with ldflags).
	BuildTime string

	// CommitHash is the Git hash of the branch, used for version purposes (set externally with ldflags).
	CommitHash = "undefined"
)

func setupVersionCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Show application version",
		Long:  "Use version command to show the application version.",
		RunE:  versionCommandAction,
	}
	return cmd
}

func versionCommandAction(cmd *cobra.Command, args []string) error {
	t, err := formattedBuildTime()
	if err != nil {
		return errors.Wrap(err, "formatting build time failed")
	}
	cmd.Printf("elastic-package version-hash %s (build time: %s)\n", CommitHash, t)
	return nil
}

func formattedBuildTime() (string, error) {
	if BuildTime == "" {
		return "unknown", nil
	}

	seconds, err := strconv.ParseInt(BuildTime, 10, 64)
	if err != nil {
		return "", errors.Wrap(err, "parsing build time failed")
	}
	return time.Unix(seconds, 0).Format(time.RFC3339), nil
}
