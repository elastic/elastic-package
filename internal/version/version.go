// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package version

import (
	"runtime/debug"
	"strconv"
	"time"
)

var (
	// BuildTime is the build time of the binary (set externally with ldflags).
	BuildTime = "unknown"

	// CommitHash is the Git hash of the branch, used for version purposes (set externally with ldflags).
	CommitHash = "undefined"

	// Tag describes the semver version of the application (set externally with ldflags).
	Tag string
)

// Set Tag to version stored in modinfo if it is not available from the builder.
func init() {
	if Tag != "" {
		return
	}
	info, ok := debug.ReadBuildInfo()
	if ok && info.Main.Version != "(devel)" {
		Tag = info.Main.Version
	}
}

// BuildTimeFormatted method returns the build time preserving the RFC3339 format.
func BuildTimeFormatted() string {
	if BuildTime == "unknown" {
		return BuildTime
	}

	seconds, err := strconv.ParseInt(BuildTime, 10, 64)
	if err != nil {
		return "invalid"
	}
	return time.Unix(seconds, 0).Format(time.RFC3339)
}
