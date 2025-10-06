// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package kibana

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"sort"
	"strings"

	"github.com/elastic/elastic-package/internal/common"
	"github.com/elastic/elastic-package/internal/logger"
)

const findDashboardsPerPage = 100

// DashboardSavedObject corresponds to the Kibana dashboard saved object
type DashboardSavedObject struct {
	ID    string
	Title string
}

// DashboardSavedObjects is an array of DashboardSavedObject
type DashboardSavedObjects []DashboardSavedObject

type savedObjectsResponse struct {
	Total        int
	SavedObjects []savedObjectResponse `json:"saved_objects"`

	Error   string
	Message string
}

type savedObjectResponse struct {
	ID         string
	Attributes struct {
		Title string
	}
}

// Strings method returns string representation for a set of saved objects.
func (dsos DashboardSavedObjects) Strings() []string {
	var entries []string
	for _, dso := range dsos {
		entries = append(entries, dso.String())
	}
	return entries
}

// String method returns a string representation for Kibana dashboard saved object.
func (dso *DashboardSavedObject) String() string {
	return fmt.Sprintf("%s (ID: %s)", dso.Title, dso.ID)
}

// FindDashboards method returns dashboards available in the Kibana instance.
func (c *Client) FindDashboards(ctx context.Context) (DashboardSavedObjects, error) {
	logger.Debug("Find dashboards using the Saved Objects API")

	var foundObjects DashboardSavedObjects
	page := 1

	for {
		r, err := c.findDashboardsNextPage(ctx, page)
		if err != nil {
			return nil, fmt.Errorf("can't fetch page with results: %w", err)
		}
		if r.Error != "" {
			return nil, fmt.Errorf("%s: %s", r.Error, r.Message)
		}

		for _, savedObject := range r.SavedObjects {
			foundObjects = append(foundObjects, DashboardSavedObject{
				ID:    savedObject.ID,
				Title: savedObject.Attributes.Title,
			})
		}

		if r.Total <= len(foundObjects) {
			break
		}
		page++
	}

	sort.Slice(foundObjects, func(i, j int) bool {
		return sort.StringsAreSorted([]string{strings.ToLower(foundObjects[i].Title), strings.ToLower(foundObjects[j].Title)})
	})
	return foundObjects, nil
}

func (c *Client) findDashboardsNextPage(ctx context.Context, page int) (*savedObjectsResponse, error) {
	path := fmt.Sprintf("%s/_find?type=dashboard&fields=title&per_page=%d&page=%d", SavedObjectsAPI, findDashboardsPerPage, page)
	statusCode, respBody, err := c.get(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("could not find dashboards; API status code = %d; response body = %s: %w", statusCode, string(respBody), err)
	}

	if statusCode != http.StatusOK {
		return nil, fmt.Errorf("could not find dashboards; API status code = %d; response body = %s", statusCode, string(respBody))
	}

	var r savedObjectsResponse
	err = json.Unmarshal(respBody, &r)
	if err != nil {
		return nil, fmt.Errorf("unmarshalling response failed: %w", err)
	}
	return &r, nil
}

// SetManagedSavedObject method sets the managed property in a saved object.
// For example managed dashboards cannot be edited, and setting managed to false will
// allow to edit them.
// Managed property cannot be directly changed, so we modify it by exporting the
// saved object and importing it again, overwriting the original one.
func (c *Client) SetManagedSavedObject(ctx context.Context, savedObjectType string, id string, managed bool) error {
	exportRequest := ExportSavedObjectsRequest{
		ExcludeExportDetails:  true,
		IncludeReferencesDeep: false,
		Objects: []ExportSavedObjectsRequestObject{
			{
				ID:   id,
				Type: savedObjectType,
			},
		},
	}
	objects, err := c.ExportSavedObjects(ctx, exportRequest)
	if err != nil {
		return fmt.Errorf("failed to export %s %s: %w", savedObjectType, id, err)
	}

	for _, o := range objects {
		o["managed"] = managed
	}

	importRequest := ImportSavedObjectsRequest{
		Overwrite: true,
		Objects:   objects,
	}
	_, err = c.ImportSavedObjects(ctx, importRequest)
	if err != nil {
		return fmt.Errorf("failed to import %s %s: %w", savedObjectType, id, err)
	}

	return nil
}

type ExportSavedObjectsRequest struct {
	Type                  string                            `json:"type,omitempty"`
	ExcludeExportDetails  bool                              `json:"excludeExportDetails"`
	IncludeReferencesDeep bool                              `json:"includeReferencesDeep"`
	Objects               []ExportSavedObjectsRequestObject `json:"objects,omitempty"`
}

type ExportSavedObjectsRequestObject struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}

func (c *Client) ExportSavedObjects(ctx context.Context, request ExportSavedObjectsRequest) ([]common.MapStr, error) {
	body, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to encode request: %w", err)
	}

	path := SavedObjectsAPI + "/_export"
	statusCode, respBody, err := c.SendRequest(ctx, http.MethodPost, path, body)
	if err != nil {
		return nil, fmt.Errorf("could not export saved objects; API status code = %d; response body = %s: %w", statusCode, string(respBody), err)
	}
	if statusCode != http.StatusOK {
		return nil, fmt.Errorf("could not export saved objects; API status code = %d; response body = %s", statusCode, string(respBody))
	}

	var objects []common.MapStr
	decoder := json.NewDecoder(bytes.NewReader(respBody))
	for decoder.More() {
		var object map[string]any
		err := decoder.Decode(&object)
		if err != nil {
			return nil, fmt.Errorf("unmarshalling response failed (body: \n%s): %w", string(respBody), err)
		}

		objects = append(objects, object)
	}

	return objects, nil
}

type ImportSavedObjectsRequest struct {
	Overwrite bool
	Objects   []common.MapStr
}

type ImportSavedObjectsResponse struct {
	Success bool           `json:"success"`
	Count   int            `json:"successCount"`
	Results []ImportResult `json:"successResults"`
	Errors  []ImportResult `json:"errors"`
}

type ImportResult struct {
	ID    string         `json:"id"`
	Type  string         `json:"type"`
	Title string         `json:"title"`
	Error map[string]any `json:"error"`
	Meta  map[string]any `json:"meta"`
}

func (c *Client) ImportSavedObjects(ctx context.Context, importRequest ImportSavedObjectsRequest) (*ImportSavedObjectsResponse, error) {
	var body bytes.Buffer
	multipartWriter := multipart.NewWriter(&body)
	fileWriter, err := multipartWriter.CreateFormFile("file", "file.ndjson")
	if err != nil {
		return nil, fmt.Errorf("failed to create multipart form file: %w", err)
	}
	enc := json.NewEncoder(fileWriter)
	for _, object := range importRequest.Objects {
		// Encode includes the newline delimiter.
		err := enc.Encode(object)
		if err != nil {
			return nil, fmt.Errorf("failed to encode object as json: %w", err)
		}
	}
	err = multipartWriter.Close()
	if err != nil {
		return nil, fmt.Errorf("failed to finalize multipart message: %w", err)
	}

	path := SavedObjectsAPI + "/_import"
	request, err := c.newRequest(ctx, http.MethodPost, path, &body)
	if err != nil {
		return nil, fmt.Errorf("cannot create new request: %w", err)
	}
	request.Header.Set("Content-Type", multipartWriter.FormDataContentType())
	if importRequest.Overwrite {
		q := request.URL.Query()
		q.Set("overwrite", "true")
		request.URL.RawQuery = q.Encode()
	}

	statusCode, respBody, err := c.doRequest(request)
	if err != nil {
		return nil, fmt.Errorf("could not import saved objects; API status code = %d; response body = %s: %w", statusCode, string(respBody), err)
	}
	if statusCode != http.StatusOK {
		return nil, fmt.Errorf("could not import saved objects; API status code = %d; response body = %s", statusCode, string(respBody))
	}

	var results ImportSavedObjectsResponse
	err = json.Unmarshal(respBody, &results)
	if err != nil {
		return nil, fmt.Errorf("could not decode response; response body: %s: %w", respBody, err)
	}
	return &results, nil
}
