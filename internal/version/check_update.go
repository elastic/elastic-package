// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package version

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"

	"github.com/elastic/elastic-package/internal/configuration/locations"
	"github.com/elastic/elastic-package/internal/environment"
	"github.com/elastic/elastic-package/internal/github"
)

const (
	repositoryOwner = "elastic"
	repositoryName  = "elastic-package"

	latestVersionFile    = "latestVersion"
	defaultCacheDuration = 30 * time.Minute
)

var checkUpdatedDisabledEnv = environment.WithElasticPackagePrefix("CHECK_UPDATE_DISABLED")

type versionLatest struct {
	TagName   string    `json:"tag"`
	HtmlURL   string    `json:"html_url"`
	Timestamp time.Time `json:"timestamp"`
}

func (v versionLatest) String() string {
	return fmt.Sprintf("%s. Download from: %s (Timestamp %s)", v.TagName, v.HtmlURL, v.Timestamp)
}

// CheckUpdate function checks using Github Release API if newer version is available.
func CheckUpdate(ctx context.Context, logger *slog.Logger) {
	if Tag == "" {
		logger.Debug("Distribution built without a version tag, can't determine release chronology. Please consider using official releases at " +
			"https://github.com/elastic/elastic-package/releases")
		return
	}

	v, ok := os.LookupEnv(checkUpdatedDisabledEnv)
	if ok && strings.ToLower(v) != "false" {
		logger.Debug("Disabled checking updates")
		return
	}

	expired := true
	latestVersion, err := loadCacheLatestVersion(logger)
	switch {
	case err != nil:
		logger.Debug("failed to load latest version from cache", slog.Any("error", err))
	default:
		expired = checkCachedLatestVersion(latestVersion, defaultCacheDuration)
	}

	var release *versionLatest
	switch {
	case !expired:
		logger.Debug("latest version (cached)", slog.String("version", latestVersion.String()))
		release = latestVersion
	default:
		logger.Debug("checking latest release in Github")
		githubClient := github.UnauthorizedClient()
		githubRelease, err := githubClient.LatestRelease(ctx, repositoryOwner, repositoryName)
		if err != nil {
			logger.Debug("failed to get latest release", slog.Any("error", err))
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
		logger.Debug("Error: can't parse current version tag", slog.String("tag", Tag), slog.Any("error", err))
		return
	}

	releaseVersion, err := semver.NewVersion(release.TagName[1:]) // strip "v" prefix
	if err != nil {
		logger.Debug("Error: can't parse current version tag", slog.String("tag", release.TagName), slog.Any("error", err))
		return
	}

	if currentVersion.LessThan(releaseVersion) {
		logger.Info("New version is available", slog.String("current", Tag), slog.String("version", release.TagName), slog.String("download_url", release.HtmlURL))
	}

	// if version cached is not expired, do not write contents into file
	if !expired {
		return
	}

	if err := writeLatestReleaseToCache(release); err != nil {
		logger.Debug("failed to write latest versoin to cache file", slog.Any("error", err))
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

func loadCacheLatestVersion(logger *slog.Logger) (*versionLatest, error) {
	elasticPackagePath, err := locations.NewLocationManager()
	if err != nil {
		return nil, fmt.Errorf("failed locating the configuration directory: %w", err)
	}

	latestVersionPath := filepath.Join(elasticPackagePath.RootDir(), latestVersionFile)
	contents, err := os.ReadFile(latestVersionPath)
	if err != nil {
		logger.Warn("reading version file failed", slog.Any("error", err))
		return nil, fmt.Errorf("reading version file failed: %w", err)
	}

	var infoVersion versionLatest
	err = json.Unmarshal(contents, &infoVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to decode file %s: %w", latestVersionPath, err)
	}

	return &infoVersion, nil
}

func checkCachedLatestVersion(latest *versionLatest, expiration time.Duration) bool {
	exprirationTime := time.Now().Add(-expiration)

	return latest.Timestamp.Before(exprirationTime)
}
