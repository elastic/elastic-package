// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package docs

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateReadme(t *testing.T) {
	cases := []struct {
		title                  string
		packageRoot            string
		filename               string
		readmeTemplateContents string
		expected               string
	}{
		{
			title:       "Pure markdown",
			packageRoot: t.TempDir(),
			filename:    "README.md",
			readmeTemplateContents: `
# README
Introduction to the package`,
			expected: `
# README
Introduction to the package`,
		},
		{
			title:                  "Static README",
			packageRoot:            t.TempDir(),
			filename:               "README.md",
			readmeTemplateContents: "",
			expected:               "",
		},
	}
	for _, c := range cases {
		t.Run(c.title, func(t *testing.T) {
			err := createReadmeFile(c.packageRoot, c.readmeTemplateContents)
			require.NoError(t, err)

			rendered, isTemplate, err := generateReadme(c.filename, c.packageRoot)
			require.NoError(t, err)

			if c.readmeTemplateContents != "" {
				renderedString := string(rendered)
				assert.True(t, isTemplate)
				assert.Equal(t, c.expected, renderedString)
			} else {
				assert.False(t, isTemplate)
				assert.Nil(t, rendered)
			}
		})
	}
}

func TestRenderReadmeWithLinks(t *testing.T) {
	minimumLinksMap := newLinkMap()
	minimumLinksMap.Add("foo", "http://www.example.com/bar")

	cases := []struct {
		title                  string
		packageRoot            string
		templatePath           string
		readmeTemplateContents string
		linksMap               linkMap
		expected               string
	}{
		{
			title:        "Readme with url function",
			packageRoot:  t.TempDir(),
			templatePath: "_dev/build/docs/README.md",
			readmeTemplateContents: `
# README
Introduction to the package
{{ url "foo" }}
{{ url "foo" "Example" }}`,
			expected: `
# README
Introduction to the package
http://www.example.com/bar
[Example](http://www.example.com/bar)`,
			linksMap: minimumLinksMap,
		},
	}

	for _, c := range cases {
		t.Run(c.title, func(t *testing.T) {
			filename := filepath.Base(c.templatePath)
			templatePath := filepath.Join(c.packageRoot, c.templatePath)

			err := createReadmeFile(c.packageRoot, c.readmeTemplateContents)
			require.NoError(t, err)

			rendered, err := renderReadme(filename, c.packageRoot, templatePath, c.linksMap)
			require.NoError(t, err)

			renderedString := string(rendered)
			assert.Equal(t, c.expected, renderedString)
		})
	}
}

func TestRenderReadmeWithSampleEvent(t *testing.T) {
	cases := []struct {
		title                   string
		packageRoot             string
		templatePath            string
		dataStreamName          string
		readmeTemplateContents  string
		sampleEventJsonContents string
		expected                string
	}{
		{
			title:        "README with sample event",
			packageRoot:  t.TempDir(),
			templatePath: "_dev/build/docs/README.md",
			readmeTemplateContents: `
# README
Introduction to the package
{{ event "example" }}`,
			expected: `
# README
Introduction to the package
An example event for ` + "`example`" + ` looks as following:

` + "```json" + `
{
    "id": "event1"
}
` + "```",
			dataStreamName:          "example",
			sampleEventJsonContents: `{"id": "event1"}`,
		},
	}

	linksMap := newLinkMap()
	for _, c := range cases {
		t.Run(c.title, func(t *testing.T) {
			filename := filepath.Base(c.templatePath)
			templatePath := filepath.Join(c.packageRoot, c.templatePath)

			err := createReadmeFile(c.packageRoot, c.readmeTemplateContents)
			require.NoError(t, err)

			err = createSampleEventFile(c.packageRoot, c.dataStreamName, c.sampleEventJsonContents)
			require.NoError(t, err)

			rendered, err := renderReadme(filename, c.packageRoot, templatePath, linksMap)
			require.NoError(t, err)

			renderedString := string(rendered)
			assert.Equal(t, c.expected, renderedString)
		})
	}
}

func TesRenderReadmeWithFields(t *testing.T) {
	cases := []struct {
		title                  string
		packageRoot            string
		templatePath           string
		dataStreamName         string
		readmeTemplateContents string
		fieldsContents         string
		expected               string
	}{
		{
			title:        "README fields from package",
			packageRoot:  t.TempDir(),
			templatePath: "_dev/build/docs/README.md",
			readmeTemplateContents: `
# README
Introduction to the package
{{ fields }}`,
			expected: `
# README
Introduction to the package
**Exported fields**

| Field | Description | Type |
|---|---|---|
| data_stream.type | Data stream type package. | constant_keyword |
`,
			dataStreamName: "",
			fieldsContents: `
- name: data_stream.type
  type: constant_keyword
  description: Data stream type package.`,
		},
		{
			title:        "README with one field",
			packageRoot:  t.TempDir(),
			templatePath: "_dev/build/docs/README.md",
			readmeTemplateContents: `
# README
Introduction to the package
{{ fields "example" }}`,
			expected: `
# README
Introduction to the package
**Exported fields**

| Field | Description | Type |
|---|---|---|
| data_stream.type | Data stream type. | constant_keyword |
`,
			dataStreamName: "example",
			fieldsContents: `
- name: data_stream.type
  type: constant_keyword
  description: Data stream type.`,
		},
		{
			title:        "README no fields",
			packageRoot:  t.TempDir(),
			templatePath: "_dev/build/docs/README.md",
			readmeTemplateContents: `
# README
Introduction to the package
{{ fields "notexist" }}`,
			expected: `
# README
Introduction to the package
**Exported fields**

(no fields available)
`,
			dataStreamName: "example",
			fieldsContents: "",
		},
	}

	linksMap := newLinkMap()
	for _, c := range cases {
		t.Run(c.title, func(t *testing.T) {
			filename := filepath.Base(c.templatePath)
			templatePath := filepath.Join(c.packageRoot, c.templatePath)

			err := createReadmeFile(c.packageRoot, c.readmeTemplateContents)
			require.NoError(t, err)

			err = createFieldsFile(c.packageRoot, c.dataStreamName, c.fieldsContents)
			require.NoError(t, err)

			rendered, err := renderReadme(filename, c.packageRoot, templatePath, linksMap)
			require.NoError(t, err)

			renderedString := string(rendered)
			assert.Equal(t, c.expected, renderedString)
		})
	}
}

func createReadmeFile(packageRoot, contents string) error {
	docsFolder, err := createDocsFolder(packageRoot)
	if err != nil {
		return err
	}

	if contents != "" {
		readmeFile := filepath.Join(docsFolder, "README.md")
		os.WriteFile(readmeFile, []byte(contents), 0644)
	}
	return nil
}

func createDocsFolder(packageRoot string) (string, error) {
	docsFolder := filepath.Join(packageRoot, "_dev", "build", "docs")
	err := os.MkdirAll(docsFolder, os.ModePerm)
	if err != nil {
		return "", err
	}
	return docsFolder, nil
}

func createSampleEventFile(packageRoot, dataStreamName, contents string) error {
	dataStreamFolder, err := createDataStreamFolder(packageRoot, dataStreamName)
	if err != nil {
		return err
	}

	sampleEventFile := filepath.Join(dataStreamFolder, sampleEventFile)
	if err := os.WriteFile(sampleEventFile, []byte(contents), 0644); err != nil {
		return err
	}
	return nil
}

func createDataStreamFolder(packageRoot, dataStreamName string) (string, error) {
	if dataStreamName == "" {
		return "", nil
	}

	dataStreamFolder := filepath.Join(packageRoot, "data_stream", dataStreamName)
	if err := os.MkdirAll(dataStreamFolder, os.ModePerm); err != nil {
		return "", err
	}
	return dataStreamFolder, nil
}

func createFieldsFile(packageRoot, dataStreamName, contents string) error {
	fieldsFolder, err := createFieldsFolder(packageRoot, dataStreamName)
	if err != nil {
		return err
	}
	fieldsFile := filepath.Join(fieldsFolder, "fields.yml")
	if err := os.WriteFile(fieldsFile, []byte(contents), 0644); err != nil {
		return err
	}
	return nil
}

func createFieldsFolder(packageRoot, dataStreamName string) (string, error) {
	fieldsFolder := packageRoot
	if dataStreamName != "" {
		fieldsFolder = filepath.Join(fieldsFolder, "data_stream", dataStreamName)
	}
	fieldsFolder = filepath.Join(fieldsFolder, "fields")

	if err := os.MkdirAll(fieldsFolder, os.ModePerm); err != nil {
		return "", err
	}
	return fieldsFolder, nil
}
