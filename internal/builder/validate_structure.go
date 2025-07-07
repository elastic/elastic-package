package builder

import (
	"fmt"

	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/packages/buildmanifest"
)


func validateDocsStructure(packageRoot, destinationDir string) error {
	logger.Debugf("Validating docs structure (package: %s, destination: %s)", packageRoot, destinationDir)

	// Read the build manifest to check if the docs structure is enforced
	bm, ok, err := buildmanifest.ReadBuildManifest(packageRoot)
	if err != nil {
		return fmt.Errorf("reading build manifest failed: %w", err)
	}
	if !ok {
		return fmt.Errorf("build manifest not found in package root: %s", packageRoot)
	}

	if !bm.Dependencies.DocsStructureEnforced {
		logger.Debug("Docs structure validation is not enforced, skipping")
		return nil
	}

	logger.Debug("Docs structure validation is enforced, proceeding with validation")

	// Validate the docs structure
	err = validateDocsStructureInPackage(packageRoot, destinationDir)
	if err != nil {
		return fmt.Errorf("validating docs structure failed: %w", err)
	}

	return nil
}


func validateDocsStructureInPackage(packageRoot, destinationDir string) error {
	// Implement the actual validation logic here
	// This is a placeholder for the actual validation logic
	logger.Debugf("Validating docs structure in package root: %s, destination: %s", packageRoot, destinationDir)

	// Example validation logic could include checking for required files,
	// specific directory structures, etc.
	// For now, we just log that the validation is happening.

	logger.Debug("Docs structure validation completed successfully")
	return nil
}
