// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package archetype

const dataStreamFieldsBaseTemplate = `- name: data_stream.type
  type: constant_keyword
  description: Data stream type.
- name: data_stream.dataset
  type: constant_keyword
  description: Data stream dataset.
- name: data_stream.namespace
  type: constant_keyword
  description: Data stream namespace.
- name: '@timestamp'
  type: date
  description: Event timestamp.
`
