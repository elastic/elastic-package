// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package profile

import (
	"errors"
	"fmt"
	"os"
	"strconv"

	"github.com/elastic/go-resource"
)

func (p *Profile) Migrate(version uint) error {
	resourceManager := resource.NewManager()
	resourceManager.AddFacter(resource.StaticFacter{
		"creation_date": p.metadata.DateCreated.Format(dateFormat),
		"profile_name":  p.ProfileName,
		"profile_path":  p.ProfilePath,
	})
	resourceManager.RegisterProvider("file", &resource.FileProvider{
		Prefix: p.ProfilePath,
	})

	migrator := resource.NewMigrator(newProfileVersioner(p, resourceManager))
	migrator.AddMigration(1, func(m *resource.Manager) (resource.ApplyResults, error) {
		renames := []struct {
			oldPath string
			newPath string
		}{
			{
				oldPath: p.Path("stack", "kibana_healthcheck.sh"),
				newPath: p.Path("stack", "kibana-healthcheck.sh"),
			},
			{
				oldPath: p.Path("stack", "snapshot.yml"),
				newPath: p.Path("stack", "docker-compose.yml"),
			},
		}
		for _, rename := range renames {
			err := os.Rename(rename.oldPath, rename.newPath)
			if err == nil || errors.Is(err, os.ErrNotExist) {
				continue
			}
			if err != nil {
				return nil, fmt.Errorf("failed to move %s to %s", rename.oldPath, rename.newPath)
			}
		}
		return nil, nil
	})

	_, err := migrator.RunMigrations(resourceManager)
	if err != nil {
		return err
	}

	return nil
}

type profileVersioner struct {
	profile *Profile
	manager *resource.Manager
}

func newProfileVersioner(p *Profile, m *resource.Manager) *profileVersioner {
	return &profileVersioner{
		profile: p,
		manager: m,
	}
}

func (v *profileVersioner) Current() uint {
	metadataVersion := v.profile.metadata.Version
	version, err := strconv.Atoi(metadataVersion)
	if err != nil || version < 0 {
		return 0
	}
	return uint(version)
}

func (p *profileVersioner) Set(version uint) error {
	if version != CurrentVersion {
		return fmt.Errorf("cannot set metadata version distinct to current version, found %d", version)
	}
	_, err := p.manager.Apply(resource.Resources{
		&resource.File{
			Path:    PackageProfileMetaFile,
			Content: profileMetadataContent,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to store new profile metadata: %w", err)
	}
	p.profile.metadata.Version = strconv.Itoa(int(version))
	return nil
}
