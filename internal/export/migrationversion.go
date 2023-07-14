// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package export

import (
	"github.com/Masterminds/semver/v3"
	"github.com/elastic/elastic-package/internal/common"
)

// fixMigrationVersion forces the use of 8.7.0 migrations for objects
// exported from 8.8.0.
// XXX: this is a hack, intended to workaround https://github.com/elastic/elastic-package/issues/1354
func fixMigrationVersion(_ *transformationContext, object common.MapStr) (common.MapStr, error) {
	migrationVersion, err := getMigrationVersion(object)
	if err == common.ErrKeyNotFound {
		return object, nil
	}
	if err != nil {
		return object, err
	}

	if migrationVersion.LessThan(semver.MustParse("8.8.0")) {
		return object, nil
	}

	objectType, _ := object.GetValue("type")
	if o, ok := objectType.(string); !ok || o != "dashboard" {
		return object, nil
	}

	object.Put("coreMigrationVersion", "8.7.0")
	object.Put("migrationVersion", common.MapStr{
		objectType.(string): "8.7.0",
	})

	return object, nil
}

func getMigrationVersion(object common.MapStr) (*semver.Version, error) {
	v, err := object.GetValue("coreMigrationVersion")
	if err != nil {
		return nil, err
	}

	vStr, ok := v.(string)
	if !ok {
		return nil, common.ErrKeyNotFound
	}

	return semver.NewVersion(vStr)
}
