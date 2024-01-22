// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package testrunner

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCoberturaCoverage_Merge(t *testing.T) {
	tests := []struct {
		name               string
		rhs, lhs, expected CoberturaCoverage
		wantErr            bool
	}{
		{
			name: "merge sources",
			rhs: CoberturaCoverage{
				Sources: []*CoberturaSource{
					{Path: "/a"},
					{Path: "/c"},
				},
			},
			lhs: CoberturaCoverage{
				Sources: []*CoberturaSource{
					{Path: "/b"},
					{Path: "/c"},
				},
			},
			expected: CoberturaCoverage{
				Sources: []*CoberturaSource{
					{Path: "/a"},
					{Path: "/c"},
					{Path: "/b"},
				},
			},
		},
		{
			name: "merge packages and classes",
			rhs: CoberturaCoverage{
				Packages: []*CoberturaPackage{
					{
						Name: "a",
						Classes: []*CoberturaClass{
							{Name: "a.a"},
							{Name: "a.b"},
						},
					},
					{
						Name: "b",
						Classes: []*CoberturaClass{
							{Name: "b.a"},
						},
					},
				},
			},
			lhs: CoberturaCoverage{
				Packages: []*CoberturaPackage{
					{
						Name: "c",
						Classes: []*CoberturaClass{
							{Name: "a.a"},
						},
					},
					{
						Name: "b",
						Classes: []*CoberturaClass{
							{Name: "b.a"},
							{Name: "b.b"},
						},
					},
				},
			},
			expected: CoberturaCoverage{
				Packages: []*CoberturaPackage{
					{
						Name: "a",
						Classes: []*CoberturaClass{
							{Name: "a.a"},
							{Name: "a.b"},
						},
					},
					{
						Name: "b",
						Classes: []*CoberturaClass{
							{Name: "b.a"},
							{Name: "b.b"},
						},
					},
					{
						Name: "c",
						Classes: []*CoberturaClass{
							{Name: "a.a"},
						},
					},
				},
			},
		},
		{
			name: "merge methods and lines",
			rhs: CoberturaCoverage{
				Packages: []*CoberturaPackage{
					{
						Name: "a",
						Classes: []*CoberturaClass{
							{
								Name: "a.a",
								Methods: []*CoberturaMethod{
									{
										Name: "foo",
										Lines: []*CoberturaLine{
											{
												Number: 13,
												Hits:   2,
											},
											{
												Number: 14,
												Hits:   2,
											},
										},
									},
									{
										Name: "bar",
										Lines: []*CoberturaLine{
											{
												Number: 24,
												Hits:   1,
											},
										},
									},
								},
								Lines: []*CoberturaLine{
									{
										Number: 13,
										Hits:   2,
									},
									{
										Number: 14,
										Hits:   2,
									},
									{
										Number: 24,
										Hits:   1,
									},
								},
							},
						},
					},
				},
			},
			lhs: CoberturaCoverage{
				Packages: []*CoberturaPackage{
					{
						Name: "a",
						Classes: []*CoberturaClass{
							{
								Name: "a.a",
								Methods: []*CoberturaMethod{
									{
										Name: "foo",
										Lines: []*CoberturaLine{
											{
												Number: 13,
												Hits:   1,
											},
											{
												Number: 14,
												Hits:   1,
											},
										},
									},
									{
										Name: "bar",
										Lines: []*CoberturaLine{
											{
												Number: 24,
												Hits:   1,
											},
										},
									},
								},
								Lines: []*CoberturaLine{
									{
										Number: 13,
										Hits:   1,
									},
									{
										Number: 14,
										Hits:   1,
									},
									{
										Number: 24,
										Hits:   1,
									},
								},
							},
						},
					},
				},
			},
			expected: CoberturaCoverage{
				LinesCovered: 3,
				LinesValid:   3,
				Packages: []*CoberturaPackage{
					{
						Name: "a",
						Classes: []*CoberturaClass{
							{
								Name: "a.a",
								Methods: []*CoberturaMethod{
									{
										Name: "foo",
										Lines: []*CoberturaLine{
											{
												Number: 13,
												Hits:   3,
											},
											{
												Number: 14,
												Hits:   3,
											},
										},
									},
									{
										Name: "bar",
										Lines: []*CoberturaLine{
											{
												Number: 24,
												Hits:   2,
											},
										},
									},
								},
								Lines: []*CoberturaLine{
									{
										Number: 13,
										Hits:   3,
									},
									{
										Number: 14,
										Hits:   3,
									},
									{
										Number: 24,
										Hits:   2,
									},
								},
							},
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
