package version

import (
	"strconv"
	"time"

	"github.com/pkg/errors"
)

var (
	// BuildTime is the build time of the binary (set externally with ldflags).
	BuildTime string

	// CommitHash is the Git hash of the branch, used for version purposes (set externally with ldflags).
	CommitHash = "undefined"
)

// BuildInfo describes the version of the application.
type BuildInfo struct {
	BuildTime  string
	CommitHash string
}

// Info method returns the application version.
func Info() (BuildInfo, error) {
	buildTime, err := formattedBuildTime()
	if err != nil {
		return BuildInfo{}, errors.Wrap(err, "formatting build time failed")
	}
	return BuildInfo{
		BuildTime:  buildTime,
		CommitHash: CommitHash,
	}, nil
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
