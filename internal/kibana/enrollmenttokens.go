// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package kibana

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

type EnrollmentToken struct {
	Active   bool   `json:"active"`
	APIKey   string `json:"api_key"`
	ID       string `json:"id"`
	Name     string `json:"name"`
	PolicyID string `json:"policy_id"`
}

// GetEnrollmentTokenForPolicyID returns an active enrollment token for a given policy ID.
// It obtains the token by returning one of the list of active tokens for this policy, or
// requesting one if there are none.
func (c *Client) GetEnrollmentTokenForPolicyID(ctx context.Context, policyID string) (string, error) {
	kuery := fmt.Sprintf("active:true and policy_id:%s", policyID)
	tokens, err := c.getEnrollmentTokens(ctx, kuery)
	if err != nil {
		return "", err
	}
	if len(tokens) == 0 {
		token, err := c.requestEnrollmentToken(ctx, policyID)
		if err != nil {
			return "", fmt.Errorf("no active enrollment token found for policy %s and failed to request one: %w", policyID, err)
		}
		if !token.Active {
			return "", fmt.Errorf("requested token %s is not active, this should not happen", token.ID)
		}
		return token.APIKey, nil
	}

	// API sorts tokens by creation date in descending order, so the first one is
	// the newest, return it.
	return tokens[0].APIKey, nil
}

func (c *Client) getEnrollmentTokens(ctx context.Context, kuery string) ([]EnrollmentToken, error) {
	var tokens []EnrollmentToken
	var resp struct {
		List    []EnrollmentToken `json:"list"`
		Items   []EnrollmentToken `json:"items"`
		Total   int               `json:"total"`
		Page    int               `json:"page"`
		PerPage int               `json:"perPage"`
	}
	for {
		values := make(url.Values)
		values.Set("page", strconv.Itoa(resp.Page+1))
		values.Set("kuery", kuery)
		resource := fmt.Sprintf("%s/enrollment_api_keys?%s", FleetAPI, values.Encode())
		statusCode, respBody, err := c.get(ctx, resource)
		if err != nil {
			return nil, fmt.Errorf("could not get enrollment tokens (query: %q): %w", values.Encode(), err)
		}
		if statusCode != http.StatusOK {
			return nil, fmt.Errorf("could not get enrollment tokens (query: %q; API status code = %d; response body = %s", values.Encode(), statusCode, respBody)
		}

		if err := json.Unmarshal(respBody, &resp); err != nil {
			return nil, fmt.Errorf("could not decode response to get enrollment tokens: %w", err)
		}

		// Tokens are listed twice, at least on some versions, get only one copy of them.
		if len(resp.List) > 0 {
			tokens = append(tokens, resp.List...)
		} else if len(resp.Items) > 0 {
			tokens = append(tokens, resp.Items...)
		}

		if resp.Page*resp.PerPage >= resp.Total {
			break
		}
	}

	return tokens, nil
}

func (c *Client) requestEnrollmentToken(ctx context.Context, policyID string) (*EnrollmentToken, error) {
	reqBody := fmt.Sprintf(`{"policy_id":"%s"}`, policyID)
	resource := fmt.Sprintf("%s/enrollment_api_keys", FleetAPI)
	statusCode, respBody, err := c.post(ctx, resource, []byte(reqBody))
	if err != nil {
		return nil, fmt.Errorf("could not request enrollment token: %w", err)
	}
	if statusCode != http.StatusOK {
		return nil, fmt.Errorf("could not request enrollment token (API status code = %d; response body = %s", statusCode, respBody)
	}

	var resp struct {
		Item   EnrollmentToken `json:"item"`
		Action string          `json:"action"`
	}
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("could not decode response to request for enrollment token: %w", err)
	}

	return &resp.Item, nil
}
