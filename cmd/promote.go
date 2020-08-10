package cmd

import (
	"github.com/elastic/elastic-package/internal/promote"
	"log"
	"strings"

	"github.com/pkg/errors"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
)

func setupPromoteCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "promote",
		Short:        "Promote the package",
		Long:         "Use promote command to move packages between stages in package-storage.",
		RunE:         promoteCommandAction,
		SilenceUsage: true,
	}
	return cmd
}

func promoteCommandAction(cmd *cobra.Command, args []string) error {
	sourceStage, destinationStage, err := promptPromotion()
	if err != nil {
		return errors.Wrap(err, "prompt for promotion failed")
	}

	newestOnly, err := promptPromoteNewestOnly()
	if err != nil {
		return errors.Wrap(err, "prompt for promoting newest revisions only failed")
	}

	sourceRepository, err := promote.CloneRepository(sourceStage)
	if err != nil {
		return errors.Wrapf(err, "cloning repository failed (branch: %s)", sourceStage)
	}

	allPackages, err := promote.ListPackages(sourceRepository, newestOnly)
	if err != nil {
		return errors.Wrapf(err, "listing packages failed (newestOnly: %t)", newestOnly)
	}

	selectedPackages, err := promptPackages(allPackages.Strings())
	if err != nil {
		return errors.Wrap(err, "prompt for package selection failed")
	}

	log.Println(sourceStage, destinationStage, selectedPackages, newestOnly)
	return nil
}

func promptPromotion() (string, string, error) {
	promotionPrompt := &survey.Select{
		Message: "Which promotion would you like to run",
		Options: []string{"snapshot - staging", "staging - production", "snapshot - production"},
		Default: "snapshot - staging",
	}

	var promotion string
	err := survey.AskOne(promotionPrompt, &promotion)
	if err != nil {
		return "", "", err
	}

	s := strings.Split(promotion, " - ")
	return s[0], s[1], nil
}

func promptPromoteNewestOnly() (bool, error) {
	newestOnly := true
	prompt := &survey.Confirm{
		Message: "Would you like to promote newest revisions only and remove older ones?",
		Default: true,
	}
	err := survey.AskOne(prompt, &newestOnly)
	if err != nil {
		return false, err
	}
	return newestOnly, nil
}

func promptPackages(allPackages []string) ([]string, error) {
	packagesPrompt := &survey.MultiSelect{
		Message: "Which packages would you like to promote",
		Options: allPackages,
		PageSize: 100,
	}

	var selected []string
	err := survey.AskOne(packagesPrompt, &selected, survey.WithValidator(survey.Required))
	if err != nil {
		return nil, err
	}
	return selected, nil
}
