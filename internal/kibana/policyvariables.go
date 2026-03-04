// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package kibana

import (
	"errors"
	"strconv"

	"github.com/elastic/elastic-package/internal/common"
	"github.com/elastic/elastic-package/internal/packages"
)

// Var represents a single variable at the package or
// data stream level, encapsulating the data type of the
// variable and it's value.
type Var struct {
	Value packages.VarValue `json:"value"`
	Type  string            `json:"type"`
}

// Vars is a collection of variables either at the package or
// data stream level.
type Vars map[string]Var

func SetKibanaVariables(definitions []packages.Variable, values common.MapStr) Vars {
	vars := Vars{}
	for _, definition := range definitions {
		// Elastic Package uses the deprecated 'inputs' array in its /api/fleet/package_policies request.
		// When using this API parameter, default values are not automatically incorporated into
		// the policy, whereas with the 'inputs' object, defaults are incorporated by the API service.
		// This means that our client must include the default values in its request to ensure correct behavior.
		val := definition.Default

		value, err := values.GetValue(definition.Name)
		if err == nil {
			val = &packages.VarValue{}
			val.Unpack(value)
		} else if errors.Is(err, common.ErrKeyNotFound) && definition.Default == nil {
			// Do not include nulls for unset variables.
			continue
		}

		vars[definition.Name] = Var{
			Type:  definition.Type,
			Value: *val,
		}
	}
	return vars
}

func SetUseAPMVariable(vars Vars, variablesToAssign common.MapStr) Vars {
	if _, found := vars["use_apm"]; found {
		return vars
	}

	useAPMData, err := variablesToAssign.GetValue("use_apm")
	if errors.Is(err, common.ErrKeyNotFound) {
		// No variable is set in the config, so it is not added
		return vars
	}

	if err != nil {
		// Error getting the variable, so it is not added
		return vars
	}

	var value packages.VarValue
	if useAPMString, ok := useAPMData.(string); ok && useAPMString != "" {
		boolValue, err := strconv.ParseBool(useAPMString)
		if err != nil {
			return vars
		}
		value.Unpack(boolValue)
	}

	if useAPM, ok := useAPMData.(bool); ok {
		value.Unpack(useAPM)
	}
	if value.Value() != nil {
		vars["use_apm"] = Var{
			Value: value,
			Type:  "boolean",
		}
	}
	return vars
}

func SetDataStreamDatasetVariable(vars Vars, variablesToAssign common.MapStr, defaultValue string) Vars {
	if _, found := vars["data_stream.dataset"]; found {
		return vars
	}

	dataStreamDatasetValue := defaultValue
	v, _ := variablesToAssign.GetValue("data_stream.dataset")
	if dataset, ok := v.(string); ok && dataset != "" {
		dataStreamDatasetValue = dataset
	}
	var value packages.VarValue
	value.Unpack(dataStreamDatasetValue)
	vars["data_stream.dataset"] = Var{
		Value: value,
		Type:  "text",
	}
	return vars
}
