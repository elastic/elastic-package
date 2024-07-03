// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package testrunner

import (
	"bytes"
	"encoding/xml"
	"fmt"
)

func init() {
	registerCoverageReporterFormat("generic")
}

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
		foundId := 0
		for idx, existingLine := range c.Lines {
			if existingLine.LineNumber == coverageLine.LineNumber {
				found = true
				foundId = idx
				break
			}
		}
		if !found {
			c.Lines = append(c.Lines, coverageLine)
		} else {
			c.Lines[foundId].Covered = c.Lines[foundId].Covered || coverageLine.Covered
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
