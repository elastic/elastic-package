// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package testrunner

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"path"
	"sort"
)

// GenericCoverage is the root element for a Cobertura XML report.
type GenericCoverage struct {
	XMLName   xml.Name       `xml:"coverage"`
	Version   int64          `xml:"version,attr"`
	Files     []*GenericFile `xml:"file"`
	Timestamp int64          `xml:"-"`
	TestType  string         `xml:",comment"`
}

type GenericFile struct {
	Path  string         `xml:"path,attr"`
	Lines []*GenericLine `xml:"lineToCover"`
}

type GenericLine struct {
	LineNumber int64 `xml:"lineNumber,attr"`
	Covered    bool  `xml:"covered,attr"`
}

func (c *GenericCoverage) TimeStamp() int64 {
	return c.Timestamp
}

func (c *GenericCoverage) Bytes() ([]byte, error) {
	out, err := xml.MarshalIndent(&c, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("unable to format test results as Coverage: %w", err)
	}

	var buffer bytes.Buffer
	buffer.WriteString(xml.Header)
	buffer.WriteString("\n")
	buffer.Write(out)
	return buffer.Bytes(), nil
}

func (c *GenericFile) merge(b *GenericFile) error {
	// Merge files
	for _, coverageLine := range b.Lines {
		found := false
		for _, existingLine := range c.Lines {
			if existingLine.LineNumber == coverageLine.LineNumber {
				found = true
				break
			}
		}
		if !found {
			c.Lines = append(c.Lines, coverageLine)
		}
	}
	return nil
}

// merge merges two coverage reports.
func (c *GenericCoverage) Merge(other CoverageReport) error {
	b, ok := other.(*GenericCoverage)
	if !ok {
		return fmt.Errorf("not able to assert report to be merged as GenericCoverage")
	}
	// Merge files
	for _, coverageFile := range b.Files {
		var target *GenericFile
		for _, existingFile := range c.Files {
			if existingFile.Path == coverageFile.Path {
				target = existingFile
				break
			}
		}
		if target != nil {
			if err := target.merge(coverageFile); err != nil {
				return err
			}
		} else {
			c.Files = append(c.Files, coverageFile)
		}
	}
	return nil
}

func transformToGenericCoverageReport(details *testCoverageDetails, baseFolder string, timestamp int64) *GenericCoverage {
	lineNumberTestType := lineNumberPerTestType(string(details.testType))
	var files []*GenericFile
	// sort data streams to ensure same ordering in coverage arrays
	sortedDataStreams := make([]string, 0, len(details.dataStreams))
	for dataStream := range details.dataStreams {
		sortedDataStreams = append(sortedDataStreams, dataStream)
	}
	sort.Strings(sortedDataStreams)

	for _, dataStream := range sortedDataStreams {
		if dataStream == "" && details.packageType == "integration" {
			continue // ignore tests running in the package context (not data stream), mostly referring to installed assets
		}
		testCases := details.dataStreams[dataStream]

		fileName := path.Join(baseFolder, details.packageName, "data_stream", dataStream, "manifest.yml")
		if dataStream == "" {
			// input package
			fileName = path.Join(baseFolder, details.packageName, "manifest.yml")
		}

		if len(testCases) == 0 {
			files = append(files, &GenericFile{
				Path:  fileName,
				Lines: []*GenericLine{{LineNumber: int64(lineNumberTestType), Covered: false}},
			})
		} else {
			files = append(files, &GenericFile{
				Path:  fileName,
				Lines: []*GenericLine{{LineNumber: int64(lineNumberTestType), Covered: true}},
			})
		}
	}

	return &GenericCoverage{
		Timestamp: timestamp,
		Version:   1,
		Files:     files,
		TestType:  fmt.Sprintf("Coverage for %s test", details.testType),
	}
}
