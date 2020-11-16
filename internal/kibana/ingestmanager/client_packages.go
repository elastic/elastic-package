package ingestmanager

import (
	"encoding/json"
	"fmt"

	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/packages"
)

// InstallPackage installs the given package in Fleet.
func (c *Client) InstallPackage(pkg packages.PackageManifest) ([]packages.Asset, error) {
	return managePackage(pkg, "install", c.post)
}

// RemovePackage removes the given package from Fleet.
func (c *Client) RemovePackage(pkg packages.PackageManifest) ([]packages.Asset, error) {
	return managePackage(pkg, "remove", c.delete)
}

func managePackage(pkg packages.PackageManifest, action string, actionFunc func(string, []byte) (int, []byte, error)) ([]packages.Asset, error) {
	path := fmt.Sprintf("epm/packages/%s-%s", pkg.Name, pkg.Version)
	statusCode, respBody, err := actionFunc(path, nil)
	if err != nil {
		return nil, errors.Wrapf(err, "could not %s package", action)
	}

	if statusCode != 200 {
		return nil, fmt.Errorf("could not %s package; API status code = %d; response body = %s", action, statusCode, respBody)
	}

	var resp struct {
		Assets []packages.Asset `json:"response"`
	}

	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, errors.Wrapf(err, "could not convert %s package (response) to JSON", action)
	}

	return resp.Assets, nil
}
