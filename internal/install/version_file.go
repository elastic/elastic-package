// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package install

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/configuration/locations"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/version"
)

func checkIfLatestVersionInstalled(elasticPackagePath *locations.LocationManager) (bool, error) {
	versionPath := filepath.Join(elasticPackagePath.RootDir(), versionFilename)
	versionFile, err := os.ReadFile(versionPath)
	if os.IsExist(err) {
		return false, nil // old version, no version file
	}
	if err != nil {
		return false, errors.Wrap(err, "reading version file failed")
	}
	v := string(versionFile)
	if version.CommitHash == "undefined" && strings.Contains(v, "undefined") {
		logger.Warnf("CommitHash is undefined, in both %s and the compiled binary, config may be out of date.", versionPath)
	}
	return buildVersionFile(version.CommitHash, version.BuildTime) == v, nil
}

func writeVersionFile(elasticPackagePath *locations.LocationManager) error {
	var err error
	err = writeStaticResource(err,
		filepath.Join(elasticPackagePath.RootDir(), versionFilename),
		buildVersionFile(version.CommitHash, version.BuildTime))
	if err != nil {
		return errors.Wrap(err, "writing static resource failed")
	}
	return nil
}

func buildVersionFile(commitHash, buildTime string) string {
	return fmt.Sprintf("%s-%s", commitHash, buildTime)
}
