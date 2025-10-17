// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package kibana

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"

	"github.com/elastic/elastic-package/internal/packages"
)

const findInstalledPackagesPerPage = 1

var ErrNotSupported error = errors.New("not supported")

type findInstalledPackagesResponse struct {
	// TODO: Should we remove this Response field?
	// Assets are here when old packages API is used (with hyphen, before 8.0).
	Response []InstalledPackage `json:"response"`

	// Assets are here when new packages API is used (with slash, since 8.0).
	Items       []InstalledPackage `json:"items"`
	Total       int                `json:"total"`
	SearchAfter []string           `json:"searchAfter"`
}

type InstalledPackages []InstalledPackage
type InstalledPackage struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// String method returns string representation for an installed package.
func (ip *InstalledPackage) String() string {
	return fmt.Sprintf("%s-%s", ip.Name, ip.Version)
}

type PackageAssetResponse struct {
	ID         string `json:"id"`
	Type       string `json:"type"`
	Attributes struct {
		Description string `json:"description"`
		Title       string `json:"title"`
	} `json:"attributes"`
}

func (p *PackageAssetResponse) String() string {
	return fmt.Sprintf("%s (ID: %s, Type: %s)", p.Attributes.Title, p.ID, p.Type)
}

// Strings method returns string representation for a set of installed packages.
func (ips InstalledPackages) Strings() []string {
	var entries []string
	for _, ip := range ips {
		entries = append(entries, ip.String())
	}
	return entries
}

// InstallPackage installs the given package in Fleet.
func (c *Client) InstallPackage(ctx context.Context, name, version string) ([]packages.Asset, error) {
	path := c.epmPackageUrl(name, version)
	reqBody := []byte(`{"force":true}`) // allows installing older versions of the package being tested

	statusCode, respBody, err := c.post(ctx, path, reqBody)
	if err != nil {
		return nil, fmt.Errorf("could not install package: %w", err)
	}

	return processResults("install", statusCode, respBody)
}

// EnsureZipPackageCanBeInstalled checks whether or not it can be installed a package using the upload API.
// This is intened to be used between 8.7.0 and 8.8.2 stack versions, and it is only safe to be run in those
// stack versions.
func (c *Client) EnsureZipPackageCanBeInstalled(ctx context.Context) error {
	path := fmt.Sprintf("%s/epm/packages", FleetAPI)

	req, err := c.newRequest(ctx, http.MethodPost, path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/zip")
	req.Header.Add("elastic-api-version", "2023-10-31")

	statusCode, respBody, err := c.doRequest(req)
	if err != nil {
		return fmt.Errorf("could not install zip package: %w", err)
	}
	switch statusCode {
	case http.StatusBadRequest:
		// If the stack allows to use the upload API, the response is like this one:
		// {
		//   "statusCode":400,
		//   "error":"Bad Request",
		//   "message":"Error during extraction of package: Error: end of central directory record signature not found. Assumed content type was application/zip, check if this matches the archive type."
		// }
		return nil
	case http.StatusForbidden:
		var resp struct {
			Message string `json:"message"`
		}
		if err := json.Unmarshal(respBody, &resp); err != nil {
			return fmt.Errorf("could not unmarhsall response to JSON: %w", err)
		}
		if resp.Message == "Requires Enterprise license" {
			return ErrNotSupported
		}
	}

	return fmt.Errorf("unexpected response (status code %d): %s", statusCode, string(respBody))
}

// InstallZipPackage installs the local zip package in Fleet.
func (c *Client) InstallZipPackage(ctx context.Context, zipFile string) ([]packages.Asset, error) {
	path := fmt.Sprintf("%s/epm/packages", FleetAPI)

	body, err := os.Open(zipFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read zip file: %w", err)
	}
	defer body.Close()

	req, err := c.newRequest(ctx, http.MethodPost, path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/zip")
	req.Header.Add("elastic-api-version", "2023-10-31")

	statusCode, respBody, err := c.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("could not install zip package: %w", err)
	}

	return processResults("zip-install", statusCode, respBody)
}

// RemovePackage removes the given package from Fleet.
func (c *Client) RemovePackage(ctx context.Context, name, version string) ([]packages.Asset, error) {
	path := c.epmPackageUrl(name, version)
	statusCode, respBody, err := c.delete(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("could not delete package: %w", err)
	}

	return processResults("remove", statusCode, respBody)
}

// FleetPackage contains information about a package in Fleet.
type FleetPackage struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Type        string `json:"type"`
	Status      string `json:"status"`
	SavedObject *struct {
		Attributes struct {
			InstalledElasticsearchAssets []packages.Asset `json:"installed_es"`
			InstalledKibanaAssets        []packages.Asset `json:"installed_kibana"`
			PackageAssets                []packages.Asset `json:"package_assets"`
		} `json:"attributes"`
	} `json:"savedObject"`
	InstallationInfo *struct {
		InstalledElasticsearchAssets []packages.Asset `json:"installed_es"`
		InstalledKibanaAssets        []packages.Asset `json:"installed_kibana"`
	} `json:"installationInfo"`
}

func (p *FleetPackage) Assets() []packages.Asset {
	var assets []packages.Asset
	if p.SavedObject != nil {
		assets = append(assets, p.SavedObject.Attributes.InstalledElasticsearchAssets...)
		assets = append(assets, p.SavedObject.Attributes.InstalledKibanaAssets...)
		return assets
	}
	// starting in 9.0.0 "savedObject" fields does not exist in the API response
	assets = append(assets, p.InstallationInfo.InstalledElasticsearchAssets...)
	assets = append(assets, p.InstallationInfo.InstalledKibanaAssets...)
	return assets
}

type ErrPackageNotFound struct {
	name string
}

func (e *ErrPackageNotFound) Error() string {
	return fmt.Sprintf("package %s not found", e.name)
}

// GetPackage obtains information about a package from Fleet.
func (c *Client) GetPackage(ctx context.Context, name string) (*FleetPackage, error) {
	path := c.epmPackageUrl(name, "")
	statusCode, respBody, err := c.get(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("could not get package: %w", err)
	}
	if statusCode == http.StatusNotFound {
		return nil, &ErrPackageNotFound{name: name}
	}
	if statusCode != http.StatusOK {
		return nil, fmt.Errorf("could not get package; API status code = %d; response body = %s", statusCode, string(respBody))
	}

	var response struct {
		// Response is here when old packages API is used (before 8.0)
		Response *FleetPackage `json:"response"`

		// Response is here when new packages API is used (since 8.0)
		Item *FleetPackage `json:"item"`
	}
	err = json.Unmarshal(respBody, &response)
	switch {
	case err != nil:
		return nil, fmt.Errorf("failed to decode package response: %w", err)
	case response.Response != nil:
		return response.Response, nil
	case response.Item != nil:
		return response.Item, nil
	default:
		return nil, fmt.Errorf("package %s not found in response: %s", name, string(respBody))
	}
}

func (c *Client) epmPackageUrl(name, version string) string {
	if version == "" {
		return fmt.Sprintf("%s/epm/packages/%s", FleetAPI, name)
	}
	switch {
	case c.semver != nil && c.semver.Major() < 8:
		return fmt.Sprintf("%s/epm/packages/%s-%s", FleetAPI, name, version)
	default:
		return fmt.Sprintf("%s/epm/packages/%s/%s", FleetAPI, name, version)
	}
}

func processResults(action string, statusCode int, respBody []byte) ([]packages.Asset, error) {
	if statusCode != http.StatusOK {
		return nil, fmt.Errorf("could not %s package; API status code = %d; response body = %s", action, statusCode, respBody)
	}

	var resp struct {
		// Assets are here when old packages API is used (with hyphen, before 8.0).
		Response []packages.Asset `json:"response"`

		// Assets are here when new packages API is used (with slash, since 8.0).
		Items []packages.Asset `json:"items"`
	}

	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("could not convert %s package (response) to JSON: %w", action, err)
	}

	if len(resp.Response) > 0 {
		return resp.Response, nil
	}

	return resp.Items, nil
}

// FindInstalledPackages retrieves the current installed packages (name and version). Response is sorted by name.
func (c *Client) FindInstalledPackages(ctx context.Context) (InstalledPackages, error) {
	var installed InstalledPackages
	searchAfter := ""

	for {
		r, err := c.findInstalledPackagesNextPage(ctx, searchAfter)
		if err != nil {
			return nil, fmt.Errorf("can't fetch page with results: %w", err)
		}
		if len(installed) >= r.Total {
			break
		}
		searchAfter = r.SearchAfter[0]
		if len(r.Response) > 0 {
			installed = append(installed, r.Response...)
		} else {
			installed = append(installed, r.Items...)
		}
	}
	sort.Slice(installed, func(i, j int) bool {
		return sort.StringsAreSorted([]string{strings.ToLower(installed[i].Name), strings.ToLower(installed[j].Name)})
	})

	return installed, nil
}

// findInstalledPackagesNextPage retrieves the next set of packages after the given searchAfter value.
func (c *Client) findInstalledPackagesNextPage(ctx context.Context, searchAfter string) (*findInstalledPackagesResponse, error) {
	path := fmt.Sprintf("%s/epm/packages/installed?perPage=%d&searchAfter=[\"%s\"]", FleetAPI, findInstalledPackagesPerPage, searchAfter)
	statusCode, respBody, err := c.get(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("could not find installed packages; API status code = %d; response body = %s: %w", statusCode, string(respBody), err)
	}

	if statusCode != http.StatusOK {
		return nil, fmt.Errorf("could not find installed packages; API status code = %d; response body = %s", statusCode, string(respBody))
	}

	var resp findInstalledPackagesResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("could not convert installed packages (response) to JSON: %w", err)
	}
	return &resp, nil
}

// GetDataFromPackageAssetIDs retrieves data, such as title and description, for the given a list of asset IDs
// using the "bulk_assets" Elastic Package Manager API endpoint. Response is sorted by title.
func (c *Client) GetDataFromPackageAssetIDs(ctx context.Context, assets []packages.Asset) ([]PackageAssetResponse, error) {
	path := fmt.Sprintf("%s/epm/bulk_assets", FleetAPI)
	type request struct {
		AssetIDs []packages.Asset `json:"assetIds"`
	}
	reqBody, err := json.Marshal(request{AssetIDs: assets})
	if err != nil {
		return nil, fmt.Errorf("could not convert assets (request) to JSON: %w", err)
	}
	statusCode, respBody, err := c.post(ctx, path, reqBody)
	if err != nil {
		return nil, fmt.Errorf("could not get assets; API status code = %d; response body = %s: %w", statusCode, string(respBody), err)
	}

	if statusCode != http.StatusOK {
		return nil, fmt.Errorf("could not get assets; API status code = %d; response body = %s", statusCode, string(respBody))
	}

	type packageAssetsResponse struct {
		Items []PackageAssetResponse `json:"items"`
	}

	var resp packageAssetsResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("could not convert get assets (response) to JSON: %w", err)
	}
	sort.Slice(resp.Items, func(i, j int) bool {
		return sort.StringsAreSorted([]string{strings.ToLower(resp.Items[i].Attributes.Title), strings.ToLower(resp.Items[j].Attributes.Title)})
	})
	return resp.Items, nil
}
