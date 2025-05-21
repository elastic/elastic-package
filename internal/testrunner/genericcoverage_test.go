// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package testrunner

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenericCoverage_Merge(t *testing.T) {
	tests := []struct {
		name               string
		rhs, lhs, expected GenericCoverage
		wantErr            bool
	}{
		{
			name: "merge files",
			rhs: GenericCoverage{
				Files: []*GenericFile{
					{
						Path: "/a",
						Lines: []*GenericLine{
							{LineNumber: 1, Covered: true},
						},
					},
					{
						Path:  "/c",
						Lines: []*GenericLine{},
					},
				},
			},
			lhs: GenericCoverage{
				Files: []*GenericFile{
					{
						Path: "/b",
						Lines: []*GenericLine{
							{LineNumber: 1, Covered: true},
						},
					},
					{
						Path: "/c",
						Lines: []*GenericLine{
							{LineNumber: 1, Covered: false},
							{LineNumber: 2, Covered: false},
						},
					},
				},
			},
			expected: GenericCoverage{
				Files: []*GenericFile{
					{
						Path: "/a",
						Lines: []*GenericLine{
							{LineNumber: 1, Covered: true},
						},
					},
					{
						Path: "/c",
						Lines: []*GenericLine{
							{LineNumber: 1, Covered: false},
							{LineNumber: 2, Covered: false},
						},
					},
					{
						Path: "/b",
						Lines: []*GenericLine{
							{LineNumber: 1, Covered: true},
						},
					},
				},
			},
		},
		{
			name: "merge files with same lines",
			rhs: GenericCoverage{
				Files: []*GenericFile{
					{
						Path: "/a",
						Lines: []*GenericLine{
							{LineNumber: 1, Covered: true},
							{LineNumber: 2, Covered: false},
							{LineNumber: 4, Covered: false},
						},
					},
					{
						Path: "/c",
						Lines: []*GenericLine{
							{LineNumber: 1, Covered: false},
							{LineNumber: 2, Covered: true},
							{LineNumber: 4, Covered: true},
						},
					},
				},
			},
			lhs: GenericCoverage{
				Files: []*GenericFile{
					{
						Path: "/a",
						Lines: []*GenericLine{
							{LineNumber: 1, Covered: true},
							{LineNumber: 2, Covered: true},
							{LineNumber: 3, Covered: false},
						},
					},
					{
						Path: "/c",
						Lines: []*GenericLine{
							{LineNumber: 1, Covered: false},
							{LineNumber: 2, Covered: false},
							{LineNumber: 3, Covered: true},
						},
					},
				},
			},
			expected: GenericCoverage{
				Files: []*GenericFile{
					{
						Path: "/a",
						Lines: []*GenericLine{
							{LineNumber: 1, Covered: true},
							{LineNumber: 2, Covered: true},
							{LineNumber: 4, Covered: false},
							{LineNumber: 3, Covered: false},
						},
					},
					{
						Path: "/c",
						Lines: []*GenericLine{
							{LineNumber: 1, Covered: false},
							{LineNumber: 2, Covered: true},
							{LineNumber: 4, Covered: true},
							{LineNumber: 3, Covered: true},
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.rhs.Merge(&tt.lhs)
			if !tt.wantErr {
				if !assert.NoError(t, err) {
					t.Fatal(err)
				}
			} else {
				if !assert.Error(t, err) {
					t.Fatal("error expected")
				}
			}
			assert.Equal(t, tt.expected, tt.rhs)
		})
	}
}
