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

const coverageDtd = `<!DOCTYPE coverage SYSTEM "http://cobertura.sourceforge.net/xml/coverage-04.dtd">`

// CoberturaCoverage is the root element for a Cobertura XML report.
type CoberturaCoverage struct {
	XMLName         xml.Name            `xml:"coverage"`
	LineRate        float32             `xml:"line-rate,attr"`
	BranchRate      float32             `xml:"branch-rate,attr"`
	Version         string              `xml:"version,attr"`
	Timestamp       int64               `xml:"timestamp,attr"`
	LinesCovered    int64               `xml:"lines-covered,attr"`
	LinesValid      int64               `xml:"lines-valid,attr"`
	BranchesCovered int64               `xml:"branches-covered,attr"`
	BranchesValid   int64               `xml:"branches-valid,attr"`
	Complexity      float32             `xml:"complexity,attr"`
	Sources         []*CoberturaSource  `xml:"sources>source"`
	Packages        []*CoberturaPackage `xml:"packages>package"`
}

// CoberturaSource represents a base path to the covered source code.
type CoberturaSource struct {
	Path string `xml:",chardata"`
}

// CoberturaPackage represents a package in a Cobertura XML report.
type CoberturaPackage struct {
	Name       string            `xml:"name,attr"`
	LineRate   float32           `xml:"line-rate,attr"`
	BranchRate float32           `xml:"branch-rate,attr"`
	Complexity float32           `xml:"complexity,attr"`
	Classes    []*CoberturaClass `xml:"classes>class"`
}

// CoberturaClass represents a class in a Cobertura XML report.
type CoberturaClass struct {
	Name       string             `xml:"name,attr"`
	Filename   string             `xml:"filename,attr"`
	LineRate   float32            `xml:"line-rate,attr"`
	BranchRate float32            `xml:"branch-rate,attr"`
	Complexity float32            `xml:"complexity,attr"`
	Methods    []*CoberturaMethod `xml:"methods>method"`
	Lines      []*CoberturaLine   `xml:"lines>line"`
}

// CoberturaMethod represents a method in a Cobertura XML report.
type CoberturaMethod struct {
	Name       string           `xml:"name,attr"`
	Signature  string           `xml:"signature,attr"`
	LineRate   float32          `xml:"line-rate,attr"`
	BranchRate float32          `xml:"branch-rate,attr"`
	Complexity float32          `xml:"complexity,attr"`
	Lines      []*CoberturaLine `xml:"lines>line"`
}

// CoberturaLine represents a source line in a Cobertura XML report.
type CoberturaLine struct {
	Number int   `xml:"number,attr"`
	Hits   int64 `xml:"hits,attr"`
}

func (c *CoberturaCoverage) TimeStamp() int64 {
	return c.Timestamp
}

func (c *CoberturaCoverage) Bytes() ([]byte, error) {
	out, err := xml.MarshalIndent(&c, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("unable to format test results as Coverage: %w", err)
	}

	var buffer bytes.Buffer
	buffer.WriteString(xml.Header)
	buffer.WriteString("\n")
	buffer.WriteString(coverageDtd)
	buffer.WriteString("\n")
	buffer.Write(out)
	return buffer.Bytes(), nil
}

// merge merges two coverage reports for a given class.
func (c *CoberturaClass) merge(b *CoberturaClass) error {
	// Check preconditions: classes should be the same.
	equal := c.Name == b.Name &&
		c.Filename == b.Filename &&
		len(c.Lines) == len(b.Lines) &&
		len(c.Methods) == len(b.Methods)
	for idx := range c.Lines {
		equal = equal && c.Lines[idx].Number == b.Lines[idx].Number
	}
	for idx := range c.Methods {
		equal = equal && c.Methods[idx].Name == b.Methods[idx].Name &&
			len(c.Methods[idx].Lines) == len(b.Methods[idx].Lines)
	}
	if !equal {
		return fmt.Errorf("merging incompatible classes: %+v != %+v", *c, *b)
	}
	// Update methods
	for idx := range b.Methods {
		for l := range b.Methods[idx].Lines {
			c.Methods[idx].Lines[l].Hits += b.Methods[idx].Lines[l].Hits
		}
	}
	// Rebuild lines
	c.Lines = nil
	for _, m := range c.Methods {
		c.Lines = append(c.Lines, m.Lines...)
	}
	return nil
}

// merge merges two coverage reports for a given package.
func (p *CoberturaPackage) merge(b *CoberturaPackage) error {
	// Merge classes
	for _, class := range b.Classes {
		var target *CoberturaClass
		for _, existing := range p.Classes {
			if existing.Name == class.Name {
				target = existing
				break
			}
		}
		if target != nil {
			if err := target.merge(class); err != nil {
				return err
			}
		} else {
			p.Classes = append(p.Classes, class)
		}
	}
	return nil
}

// merge merges two coverage reports.
func (c *CoberturaCoverage) Merge(other CoverageReport) error {
	b, ok := other.(*CoberturaCoverage)
	if !ok {
		return fmt.Errorf("not able to assert report to be merged as CoberturaCoverage")

	}
	// Merge source paths
	for _, path := range b.Sources {
		found := false
		for _, existing := range c.Sources {
			if found = existing.Path == path.Path; found {
				break
			}
		}
		if !found {
			c.Sources = append(c.Sources, path)
		}
	}

	// Merge packages
	for _, pkg := range b.Packages {
		var target *CoberturaPackage
		for _, existing := range c.Packages {
			if existing.Name == pkg.Name {
				target = existing
				break
			}
		}
		if target != nil {
			if err := target.merge(pkg); err != nil {
				return err
			}
		} else {
			c.Packages = append(c.Packages, pkg)
		}
	}

	// Recalculate global line coverage count
	c.LinesValid = 0
	c.LinesCovered = 0
	for _, pkg := range c.Packages {
		for _, cls := range pkg.Classes {
			for _, line := range cls.Lines {
				c.LinesValid++
				if line.Hits > 0 {
					c.LinesCovered++
				}
			}
		}
	}
	return nil
}

func transformToCoberturaReport(details *testCoverageDetails, baseFolder string, timestamp int64) *CoberturaCoverage {
	var classes []*CoberturaClass
	lineNumberTestType := lineNumberPerTestType(string(details.testType))

	// sort data streams to ensure same ordering in coverage arrays
	sortedDataStreams := make([]string, 0, len(details.dataStreams))
	for dataStream := range details.dataStreams {
		sortedDataStreams = append(sortedDataStreams, dataStream)
	}
	sort.Strings(sortedDataStreams)

	for _, dataStream := range sortedDataStreams {
		testCases := details.dataStreams[dataStream]

		if dataStream == "" && details.packageType == "integration" {
			continue // ignore tests running in the package context (not data stream), mostly referring to installed assets
		}

		var methods []*CoberturaMethod
		var lines []*CoberturaLine

		if len(testCases) == 0 {
			methods = append(methods, &CoberturaMethod{
				Name:  "Missing",
				Lines: []*CoberturaLine{{Number: lineNumberTestType, Hits: 0}},
			})
			lines = append(lines, []*CoberturaLine{{Number: lineNumberTestType, Hits: 0}}...)
		} else {
			methods = append(methods, &CoberturaMethod{
				Name:  "OK",
				Lines: []*CoberturaLine{{Number: lineNumberTestType, Hits: 1}},
			})
			lines = append(lines, []*CoberturaLine{{Number: lineNumberTestType, Hits: 1}}...)
		}

		fileName := path.Join(baseFolder, details.packageName, "data_stream", dataStream, "manifest.yml")
		if dataStream == "" {
			// input package
			fileName = path.Join(baseFolder, details.packageName, "manifest.yml")
		}

		aClass := &CoberturaClass{
			Name:     string(details.testType),
			Filename: fileName,
			Methods:  methods,
			Lines:    lines,
		}
		classes = append(classes, aClass)
	}

	return &CoberturaCoverage{
		Timestamp: timestamp,
		Packages: []*CoberturaPackage{
			{
				Name:    details.packageName,
				Classes: classes,
			},
		},
	}
}
