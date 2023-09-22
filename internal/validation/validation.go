package validation

import (
	"fmt"
	"io/fs"
	"os"

	"github.com/elastic/elastic-package/internal/logger"
	ve "github.com/elastic/package-spec/v2/code/go/pkg/errors"
	"github.com/elastic/package-spec/v2/code/go/pkg/errors/processors"
	"github.com/elastic/package-spec/v2/code/go/pkg/validator"
)

const configFilterPath = "_dev/filter.yml"

func ValidateFromPath(rootPath string) error {
	return validator.ValidateFromPath(rootPath)
}

func ValidateFromZip(packagePath string) error {
	return validator.ValidateFromPath(packagePath)
}

func ValidateAndFilterFromPath(rootPath string) error {
	allErrors := validator.ValidateFromPath(rootPath)
	if allErrors == nil {
		return nil
	}

	errors, err := filterErrors(allErrors, rootPath, configFilterPath)
	if err != nil {
		return err
	}
	return errors
}

func ValidateAndFilterFromZip(packagePath string) error {
	allErrors := validator.ValidateFromZip(packagePath)
	if allErrors == nil {
		return nil
	}

	errors, err := filterErrors(allErrors, packagePath, configFilterPath)
	if err != nil {
		return err
	}
	return errors
}

func filterErrors(allErrors error, rootPath, configPath string) (error, error) {
	errs, ok := allErrors.(ve.ValidationErrors)
	if !ok {
		return allErrors, nil
	}

	fsys := os.DirFS(rootPath)

	_, err := fs.Stat(fsys, configPath)
	if err != nil {
		logger.Debugf("file not found: %s", configPath)
		return allErrors, nil
	}

	config, err := processors.LoadConfigFilter(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config filter: %w", err)
	}

	filter := processors.NewFilter(config)

	filteredErrors, _, err := filter.Run(errs)
	if err != nil {
		return nil, fmt.Errorf("failed to filter errors: %w", err)
	}
	return filteredErrors, nil
}
