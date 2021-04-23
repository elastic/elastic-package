package surveyext

import (
	"github.com/Masterminds/semver"

	"github.com/elastic/elastic-package/internal/install"
)

// DefaultConstraintValue function returns a constraint
func DefaultConstraintValue() string {
	ver := semver.MustParse(install.DefaultStackVersion)
	v, _ := ver.SetPrerelease("")
	return "^" + v.String()
}
