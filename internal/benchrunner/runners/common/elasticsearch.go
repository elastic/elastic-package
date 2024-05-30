// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package common

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/elastic/elastic-package/internal/elasticsearch"
)

func CountDocsInDataStream(ctx context.Context, esapi *elasticsearch.API, dataStream string, logger *slog.Logger) (int, error) {
	resp, err := esapi.Count(
		esapi.Count.WithContext(ctx),
		esapi.Count.WithIndex(dataStream),
		esapi.Count.WithIgnoreUnavailable(true),
	)
	if err != nil {
		return 0, fmt.Errorf("could not search data stream: %w", err)
	}
	defer resp.Body.Close()

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
		logger.Debug("found hits in data stream:",
			slog.Int("hits", numHits),
			slog.String("datastream", dataStream),
			slog.String("type", results.Error.Type),
			slog.String("reason", results.Error.Reason),
			slog.Int("status", results.Status),
		)
	} else {
		logger.Debug("found hits in data stream", slog.Int("hits", numHits), slog.String("datastream", dataStream))
	}

	return numHits, nil
}
