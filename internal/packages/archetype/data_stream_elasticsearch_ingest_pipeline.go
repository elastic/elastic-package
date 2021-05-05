// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package archetype

const dataStreamElasticsearchIngestPipelineTemplate = `---
description: Pipeline for processing sample logs
processors:
- set:
    field: sample_field
    value: "1"
on_failure:
- set:
    field: error.message
    value: '` + "{{`{{ _ingest.on_failure_message }}`}}'"
