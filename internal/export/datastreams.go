// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package export

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"

	"github.com/elastic/elastic-package/internal/elasticsearch"
)

type DataStreamsResponse struct {
	DataStreams []DataStream `json:"data_streams"`
}

type DataStream struct {
	Name      string `json:"name"`
	Template  string `json:"template"`
	ILMPolicy string `json:"ilm_policy"`

	Meta DataStreamMeta `json:"_meta"`
}

type DataStreamMeta struct {
	ManagedBy string `json:"managed_by"`
	Managed   bool   `json:"managed"`
	Package   struct {
		Name string `json:"name"`
	} `json:"package"`
}

func DataStreamsForPackage(ctx context.Context, api *elasticsearch.API, packageName string) ([]DataStream, error) {

	resp, err := api.Indices.GetDataStream(
		api.Indices.GetDataStream.WithContext(ctx),
		api.Indices.GetDataStream.WithExpandWildcards("all"),

		// A second filter is done based on the metadata.
		api.Indices.GetDataStream.WithName(fmt.Sprintf("*-%s.*", packageName)),
	)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	d, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failure reading body: %w", err)
	}

	var dataStreamsResponse DataStreamsResponse
	err = json.Unmarshal(d, &dataStreamsResponse)
	if err != nil {
		return nil, fmt.Errorf("invalid response format: %w", err)
	}

	return filterPackageDataStreams(dataStreamsResponse.DataStreams, packageName), nil
}

func filterPackageDataStreams(dataStreams []DataStream, packageName string) []DataStream {
	var result []DataStream

	for _, ds := range dataStreams {
		if ds.Meta.ManagedBy == "ingest-manager" && ds.Meta.Package.Name == packageName {
			result = append(result, ds)
		}
	}

	return result
}
