// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package version

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"

	"github.com/elastic/elastic-package/internal/configuration/locations"
	"github.com/elastic/elastic-package/internal/environment"
	"github.com/elastic/elastic-package/internal/github"
	"github.com/elastic/elastic-package/internal/logger"
)

const (
	repositoryOwner = "elastic"
	repositoryName  = "elastic-package"

	latestVersionFile    = "latestVersion"
	defaultCacheDuration = "30m"
)

var checkUpdatedDisabledEnv = environment.WithElasticPackagePrefix("CHECK_UPDATE_DISABLED")

type versionLatest struct {
	// Release   gh.RepositoryRelease `json:"latest"`
	TagName   string    `json:"tag"`
	HtmlURL   string    `json:"html_url"`
	Timestamp time.Time `json:"timestamp"`
}

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

	duration, err := time.ParseDuration(defaultCacheDuration)
	if err != nil {
		logger.Warnf("failed to parse cache duration: %s", err.Error())
		return
	}
	latestVersion, expired, _ := checkCachedLatestVersion(duration)
	var release *versionLatest
	switch {
	case !expired:
		logger.Debugf("latest version (cached): %s", latestVersion)
		release = latestVersion
	default:
		logger.Debugf("checking latest release in Github")
		githubClient := github.UnauthorizedClient()
		githubRelease, err := githubClient.LatestRelease(context.TODO(), repositoryOwner, repositoryName)
		if err != nil {
			logger.Debugf("Error: %v", err)
			return
		}
		release = &versionLatest{
			TagName:   *githubRelease.TagName,
			HtmlURL:   *githubRelease.HTMLURL,
			Timestamp: time.Now(),
		}
	}

	currentVersion, err := semver.NewVersion(Tag[1:]) // strip "v" prefix
	if err != nil {
		logger.Debugf("Error: can't parse current version tag, %v", err)
		return
	}

	releaseVersion, err := semver.NewVersion(release.TagName[1:]) // strip "v" prefix
	if err != nil {
		logger.Debugf("Error: can't parse current version tag, %v", err)
		return
	}

	if currentVersion.LessThan(releaseVersion) {
		logger.Infof("New version is available - %s. Download from: %s", release.TagName, release.HtmlURL)
	}

	if err := writeLatestReleaseToCache(release); err != nil {
		logger.Debugf("failed to write latest versoin to cache file: %v", err)
	}
}

func writeLatestReleaseToCache(release *versionLatest) error {
	elasticPackagePath, err := locations.NewLocationManager()
	if err != nil {
		return fmt.Errorf("failed locating the configuration directory: %w", err)
	}

	latestVersionPath := filepath.Join(elasticPackagePath.RootDir(), latestVersionFile)

	contents, err := json.Marshal(release)
	if err != nil {
		return fmt.Errorf("failed to encode file %s: %w", latestVersionPath, err)
	}
	err = os.WriteFile(latestVersionPath, contents, 0644)
	if err != nil {
		return fmt.Errorf("writing file failed (path: %s): %w", latestVersionPath, err)
	}

	return nil
}

func checkCachedLatestVersion(expiration time.Duration) (*versionLatest, bool, error) {
	elasticPackagePath, err := locations.NewLocationManager()
	if err != nil {
		return nil, true, fmt.Errorf("failed locating the configuration directory: %w", err)
	}

	latestVersionPath := filepath.Join(elasticPackagePath.RootDir(), latestVersionFile)
	contents, err := os.ReadFile(latestVersionPath)
	if err != nil {
		logger.Warnf("reading version file failed: %w", err.Error())
		return nil, true, fmt.Errorf("reading version file failed: %w", err)
	}

	var infoVersion versionLatest
	err = json.Unmarshal(contents, &infoVersion)
	if err != nil {
		return nil, true, fmt.Errorf("failed to decode file %s: %w", latestVersionPath, err)
	}

	exprirationTime := time.Now().Add(-expiration)

	if infoVersion.Timestamp.Before(exprirationTime) {
		return nil, true, fmt.Errorf("expired version: %s (%s)", infoVersion.TagName, infoVersion.Timestamp)
	}
	return &infoVersion, false, nil
}
