// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package profile

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"time"

	"github.com/elastic/go-resource"
)

// Metadata stores the data associated with a given profile
type Metadata struct {
	Name        string    `json:"name"`
	DateCreated time.Time `json:"date_created"`
	Version     string    `json:"version"`
}

// profileMetadataContent generates the content of the profile.json file.
func profileMetadataContent(applyCtx resource.Context, w io.Writer) error {
	creationDateFormated, found := applyCtx.Fact("creation_date")
	if !found {
		return errors.New("unknown creation date")
	}
	creationDate, err := time.Parse(time.RFC3339Nano, creationDateFormated)
	if err != nil {
		return fmt.Errorf("failed to parse creation date from fact (%v): %w", creationDateFormated, err)
	}

	profileName, found := applyCtx.Fact("profile_name")
	if !found {
		return errors.New("unknown profile name")
	}

	profileData := Metadata{
		Name:        profileName,
		DateCreated: creationDate,
		Version:     strconv.Itoa(currentVersion),
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	err = enc.Encode(profileData)
	if err != nil {
		return fmt.Errorf("error marshalling json: %w", err)
	}

	return nil
}

func loadProfileMetadata(path string) (Metadata, error) {
	d, err := os.ReadFile(path)
	if err != nil {
		return Metadata{}, fmt.Errorf("error reading metadata file: %w", err)
	}

	metadata := Metadata{}
	err = json.Unmarshal(d, &metadata)
	if err != nil {
		return Metadata{}, fmt.Errorf("error checking profile metadata file %q: %w", path, err)
	}
	return metadata, nil
}
