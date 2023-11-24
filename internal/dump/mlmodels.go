// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package dump

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/elastic/elastic-package/internal/elasticsearch"
)

// MLModel contains the information needed to export a machine learning trained model.
type MLModel struct {
	ModelID string `json:"model_id"`
	raw     json.RawMessage
}

func (m MLModel) Name() string {
	return m.ModelID
}

func (m MLModel) JSON() []byte {
	return []byte(m.raw)
}

type getMLModelsResponse struct {
	Models []json.RawMessage `json:"trained_model_configs"`
}

func getMLModelsForPackage(ctx context.Context, api *elasticsearch.API, packageName string) ([]MLModel, error) {
	resp, err := api.ML.GetTrainedModels(
		api.ML.GetTrainedModels.WithContext(ctx),

		// Wildcard may be too wide, but no other thing is available at the moment.
		api.ML.GetTrainedModels.WithModelID(fmt.Sprintf("%s_*", packageName)),

		// Decompressing definition uses to OOM, keep it compressed as we do in packages.
		api.ML.GetTrainedModels.WithDecompressDefinition(false),

		// Include all the available optional information.
		api.ML.GetTrainedModels.WithInclude(strings.Join([]string{
			"definition",
			"feature_importance_baseline",
			"hyperparameters",
			"total_feature_importance",
			// Definition status not included because it only works with some models.
			// "definition_status",
		}, ",")),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get ML models: %w", err)
	}
	defer resp.Body.Close()
	if resp.IsError() {
		return nil, fmt.Errorf("failed to get ML models: %s", resp)
	}

	d, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var modelsResponse getMLModelsResponse
	err = json.Unmarshal(d, &modelsResponse)
	if err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	var models []MLModel
	for _, modelRaw := range modelsResponse.Models {
		var model MLModel
		err = json.Unmarshal(modelRaw, &model)
		if err != nil {
			return nil, fmt.Errorf("failed to parse ML model: %w", err)
		}
		model.raw = modelRaw

		models = append(models, model)
	}

	return models, nil

}
