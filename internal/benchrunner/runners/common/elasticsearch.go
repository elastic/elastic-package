// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package common

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/elastic/elastic-package/internal/elasticsearch"
	"github.com/elastic/elastic-package/internal/logger"
)

func CountDocsInDataStream(ctx context.Context, esapi *elasticsearch.API, dataStream string) (int, error) {
	resp, err := esapi.Count(
		esapi.Count.WithContext(ctx),
		esapi.Count.WithIndex(dataStream),
		esapi.Count.WithIgnoreUnavailable(true),
	)
	if err != nil {
		return 0, fmt.Errorf("could not search data stream: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusServiceUnavailable && strings.Contains(resp.String(), "no_shard_available_action_exception") {
		// Index is being created, but no shards are available yet.
		// See https://github.com/elastic/elasticsearch/issues/65846
		return 0, nil
	}

	if resp.IsError() {
		return 0, fmt.Errorf("failed to get hits count: %s", resp.String())
	}

	var results struct {
		Count int
		Error *struct {
			Type   string
			Reason string
		}
		Status int
	}

	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return 0, fmt.Errorf("could not decode search results response: %w", err)
	}

	numHits := results.Count
	if results.Error != nil {
		logger.Debugf("found %d hits in %s data stream: %s: %s Status=%d",
			numHits, dataStream, results.Error.Type, results.Error.Reason, results.Status)
	} else {
		logger.Debugf("found %d hits in %s data stream", numHits, dataStream)
	}

	return numHits, nil
}
