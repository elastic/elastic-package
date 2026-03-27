// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTrimStringSlice(t *testing.T) {
	strs := []string{"foo bar ", "  bar baz", "\tbaz qux\t\t", "qux foo"}
	expected := []string{"foo bar", "bar baz", "baz qux", "qux foo"}

	TrimStringSlice(strs)
	require.Equal(t, expected, strs)
}

func TestStringSlicesUnion(t *testing.T) {
	cases := []struct {
		slices   [][]string
		expected []string
	}{
		{nil, nil},
		{[][]string{{"foo", "bar"}, nil}, []string{"foo", "bar"}},
		{[][]string{nil, {"foo", "bar"}}, []string{"foo", "bar"}},
		{[][]string{{"foo", "bar"}, {"foo", "bar"}}, []string{"foo", "bar"}},
		{[][]string{{"foo", "baz"}, {"foo", "bar"}}, []string{"foo", "bar", "baz"}},
		{[][]string{{"foo", "bar"}, {"foo", "baz"}}, []string{"foo", "bar", "baz"}},
	}

	for _, c := range cases {
		result := StringSlicesUnion(c.slices...)
		assert.ElementsMatch(t, c.expected, result)
	}
}

func TestGCPCredentialFacters(t *testing.T) {
	cases := []struct {
		name                         string
		googleApplicationCredentials string
		expectedCredentialSourceFile string
		expectedGoogleAppCredentials string
		expectError                  bool
	}{
		{
			name:                         "file exists and is valid - no credential source",
			googleApplicationCredentials: "testdata/gcp_facters/valid_google_credentials.json",
			expectedCredentialSourceFile: "",
			expectedGoogleAppCredentials: "testdata/gcp_facters/valid_google_credentials.json",
			expectError:                  false,
		},
		{
			name:                         "file exists and is valid and contains credential source",
			googleApplicationCredentials: "testdata/gcp_facters/existing_credential_source_file.json",
			expectedCredentialSourceFile: "/tmp/credential_source_file.json",
			expectedGoogleAppCredentials: "testdata/gcp_facters/existing_credential_source_file.json",
			expectError:                  false,
		},
		{
			name:                         "file exists but is invalid",
			googleApplicationCredentials: "testdata/gcp_facters/invalid_google_credentials.json",
			expectError:                  true,
		},
		{
			name:                         "no environment variable defined",
			googleApplicationCredentials: "",
			expectedCredentialSourceFile: "",
			expectedGoogleAppCredentials: "",
			expectError:                  false,
		},
		{
			name:                         "file does not exist",
			googleApplicationCredentials: "testdata/not_existing_file.json",
			expectedCredentialSourceFile: "",
			expectedGoogleAppCredentials: "",
			expectError:                  false,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", c.googleApplicationCredentials)
			facters, err := GCPCredentialFacters()
			if c.expectError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			assert.Len(t, facters, 2)

			credentialSourceFile, ok := facters["google_credential_source_file"]
			require.True(t, ok)
			assert.Equal(t, c.expectedCredentialSourceFile, credentialSourceFile)

			googleAppCredentials, ok := facters["google_application_credentials"]
			require.True(t, ok)
			assert.Equal(t, c.expectedGoogleAppCredentials, googleAppCredentials)
		})
	}
}
