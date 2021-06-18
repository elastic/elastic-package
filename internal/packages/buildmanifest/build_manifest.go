// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package buildmanifest

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"

	"github.com/elastic/go-ucfg"
	"github.com/elastic/go-ucfg/yaml"
	"github.com/pkg/errors"
	yamlv3 "gopkg.in/yaml.v3"

	"github.com/elastic/elastic-package/internal/logger"
)

const (
	ecsSchemaRefURL = "https://api.github.com/repos/elastic/ecs/commits/master"
)

// BuildManifest defines the manifest defining the building procedure.
type BuildManifest struct {
	Dependencies Dependencies `config:"dependencies"`
}

// Dependencies define external package dependencies.
type Dependencies struct {
	ECS ECSDependency `config:"ecs"`
}

// ECSDependency defines a dependency on ECS fields.
type ECSDependency struct {
	Reference string `config:"reference"`
}

// HasDependencies function checks if there are any dependencies defined.
func (bm *BuildManifest) HasDependencies() bool {
	return bm.Dependencies.ECS.Reference != ""
}

// ReadBuildManifest function reads the package build manifest.
func ReadBuildManifest(packageRoot string) (*BuildManifest, bool, error) {
	path := buildManifestPath(packageRoot)
	cfg, err := yaml.NewConfigWithFile(path, ucfg.PathSep("."))
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil // ignore not found errors
	}
	if err != nil {
		return nil, false, errors.Wrapf(err, "reading file failed (path: %s)", path)
	}

	var bm BuildManifest
	err = cfg.Unpack(&bm)
	if err != nil {
		return nil, true, errors.Wrapf(err, "unpacking build manifest failed (path: %s)", path)
	}
	return &bm, true, nil
}

// UpdateDependencies function updates dependencies on external sources.
func UpdateDependencies(packageRoot string) error {
	bm, ok, err := ReadBuildManifest(packageRoot)
	if err != nil {
		return errors.Wrap(err, "can't update dependencies")
	}
	if !ok || !bm.HasDependencies() {
		return errors.New("package doesn't use dependency management, please define the build manifest first")
	}

	logger.Debugf("Update dependency on ECS repository")
	ecsFreshRef, err := updateReferenceToECS()
	if err != nil {
		return errors.Wrap(err, "can't update reference to ECS repository")
	}

	if bm.Dependencies.ECS.Reference == ecsFreshRef {
		logger.Debugf("Dependency on ECS repository is up-to-date")
		return nil
	}

	logger.Debugf("ECS repository has been changed (latest reference: %s), the tool will update the build manifest", ecsFreshRef)
	bm.Dependencies.ECS.Reference = ecsFreshRef

	err = writeBuildManifest(packageRoot, *bm)
	if err != nil {
		return errors.Wrap(err, "can't write the build manifest")
	}
	return nil
}

func updateReferenceToECS() (string, error) {
	resp, err := http.Get(ecsSchemaRefURL)
	if err != nil {
		return "", errors.Wrap(err, "can't download ECS schema refs")
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", errors.Wrap(err, "can't read ECS schema refs")
	}

	var latest struct {
		SHA string `json:"sha"`
	}

	err = json.Unmarshal(body, &latest)
	if err != nil {
		return "", errors.Wrap(err, "can't unmarshal ECS schema refs failed")
	}

	if latest.SHA == "" {
		return "", errors.New("missing commit SHA value")
	}
	return "git@" + latest.SHA, nil
}

func writeBuildManifest(packageRoot string, manifest BuildManifest) error {
	content, err := yamlv3.Marshal(manifest)
	if err != nil {
		return errors.Wrap(err, "can't marshal build manifest")
	}

	path := buildManifestPath(packageRoot)
	err = ioutil.WriteFile(path, content, 0644)
	if err != nil {
		return errors.Wrapf(err, "can't write build manifest (path: %s)", path)
	}
	return nil
}

func buildManifestPath(packageRoot string) string {
	return filepath.Join(packageRoot, "_dev", "build", "build.yml")
}
