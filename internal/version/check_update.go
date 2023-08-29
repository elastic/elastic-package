// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package version

import (
	"context"
	"os"
	"strings"

	"github.com/Masterminds/semver/v3"

	"github.com/elastic/elastic-package/internal/environment"
	"github.com/elastic/elastic-package/internal/github"
	"github.com/elastic/elastic-package/internal/logger"
)

const (
	repositoryOwner = "elastic"
	repositoryName  = "elastic-package"
)

var checkUpdatedDisabledEnv = environment.WithElasticPackagePrefix("CHECK_UPDATE_DISABLED")

// CheckUpdate function checks using Github Release API if newer version is available.
func CheckUpdate() {
	if Tag == "" {
		logger.Debugf("Distribution built without a version tag, can't determine release chronology. Please consider using official releases at " +
			"https://github.com/elastic/elastic-package/releases")
		return
	}

	v, ok := os.LookupEnv(checkUpdatedDisabledEnv)
	if ok && strings.ToLower(v) != "false" {
		logger.Debug("Disabled checking updates")
		return
	}

	githubClient := github.UnauthorizedClient()
	release, _, err := githubClient.Repositories.GetLatestRelease(context.TODO(), repositoryOwner, repositoryName)
	if err != nil {
		logger.Debugf("Error: can't check latest release, %v", err)
		return
	}

	if release.TagName == nil || *release.TagName == "" {
		logger.Debugf("Error: release tag is empty")
		return
	}

	currentVersion, err := semver.NewVersion(Tag[1:]) // strip "v" prefix
	if err != nil {
		logger.Debugf("Error: can't parse current version tag, %v", err)
		return
	}

	releaseVersion, err := semver.NewVersion((*release.TagName)[1:]) // strip "v" prefix
	if err != nil {
		logger.Debugf("Error: can't parse current version tag, %v", err)
		return
	}

	if currentVersion.LessThan(releaseVersion) {
		logger.Infof("New version is available - %s. Download from: %s", *release.TagName, *release.HTMLURL)
	}
}
