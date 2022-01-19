// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package ingest

import (
	"bytes"
	"encoding/json"
	"io"
	"sort"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
)

// Processor represents an ingest processor.
type Processor struct {
	// Type of processor ("set", "script", etc.)
	Type        string `yaml:"-"`
	// FirstLine is the line number where this processor definition starts
	// in the pipeline source code.
	FirstLine   int `yaml:"-"`
	// LastLine is the line number where this processor definitions end
	// in the pipeline source code.
	LastLine    int `yaml:"-"`
}

// Processors return the list of processors in an ingest pipeline.
func (p Pipeline) Processors() (procs []Processor, err error) {
	switch p.Format {
	case "yaml", "yml":
		procs, err = processorsFromYAML(p.Content)
	case "json":
		procs, err = processorsFromJSON(p.Content)
	default:
		return nil, errors.Errorf("unsupported pipeline format: %s", p.Format)
	}
	return procs, errors.Wrapf(err, "failure processing %s pipeline '%s'", p.Format, p.Filename())
}

// extract a list of processors from a pipeline definition in YAML format.
func processorsFromYAML(content []byte) (procs []Processor, err error) {
	var p struct {
		Processors []yaml.Node
	}
	if err = yaml.Unmarshal(content, &p); err != nil {
		return nil, err
	}
	for idx, entry := range p.Processors {
		if entry.Kind != yaml.MappingNode || len(entry.Content) != 2 {
			return nil, errors.Errorf("processor#%d is not a single-key map (kind:%v Content:%d)", idx, entry.Kind, len(entry.Content))
		}
		var proc Processor
		if err := entry.Content[1].Decode(&proc); err != nil {
			return nil, errors.Wrapf(err, "error decoding processor#%d configuration", idx)
		}
		if err := entry.Content[0].Decode(&proc.Type); err != nil {
			return nil, errors.Wrapf(err, "error decoding processor#%d type", idx)
		}
		proc.FirstLine = entry.Line
		proc.LastLine = lastLine(&entry)
		procs = append(procs, proc)
	}
	return procs, nil
}

// returns the last (greater) line number used by a yaml.Node.
func lastLine(node *yaml.Node) int {
	if node == nil {
		return 0
	}
	last := node.Line
	for _, inner := range node.Content {
		if line := lastLine(inner); line > last {
			last = line
		}
	}
	return last
}

// tokenStack contains the current state of parsing a JSON document.
type tokenStack []json.Token

func (s *tokenStack) Push(t json.Token) {
	*s = append(*s, t)
}

func (s *tokenStack) Pop() json.Token {
	top := s.Top()
	if n := len(*s); n > 0 {
		*s = (*s)[:n-1]
	}
	return top
}

func (s *tokenStack) PopUntil(d json.Delim) {
	for {
		switch s.Pop() {
		case d, io.EOF:
			return
		}
	}
}

func (s *tokenStack) Top() json.Token {
	if n := len(*s); n > 0 {
		return (*s)[n-1]
	}
	return io.EOF
}

func (s *tokenStack) TopIsString() bool {
	if n := len(*s); n > 0 {
		_, ok := (*s)[n-1].(string)
		return ok
	}
	return false
}

func (s *tokenStack) Equals(b tokenStack) bool {
	if len(*s) != len(b) {
		return false
	}
	for idx, tk := range *s {
		if b[idx] != tk {
			return false
		}
	}
	return true
}

var processorBasePath = tokenStack{json.Delim('{'), "processors", json.Delim('['), json.Delim('{')}

func processorsFromJSON(content []byte) (processors []Processor, err error) {
	// list of processor names in order of occurrence.
	var names []string
	// start and end offsets for each processor.
	var startPos, endPos []int
	var stack tokenStack
	decoder := json.NewDecoder(bytes.NewReader(content))

	for {
		// Read the next token and it's offset.
		tk, err := decoder.Token()
		off := int(decoder.InputOffset())
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		delim, isDelim := tk.(json.Delim)

		// If the token terminates an array or object
		// pop everything in the stack until its corresponding opening token.
		if isDelim && (delim == '}' || delim == ']') {
			stack.PopUntil(delim - 2) // `}`-2 = `{` and `]`-2 = `[`
			if stack.TopIsString() {
				stack.Pop()
			}
			// If the current stack matches a processor definition, it means this was
			// the closing token for a processor.
			if stack.Equals(processorBasePath) {
				endPos = append(endPos, off)
			}
			continue
		}
		// If the current token is not a delimiter and the stack holds a string,
		// this is a key: value pair, ignoring it.
		if !isDelim && stack.TopIsString() {
			stack.Pop()
			continue
		}
		// If the current token is a string and the current stack is the processor
		// base path, this is a key that starts a new processor.
		if str, ok := tk.(string); ok && stack.Equals(processorBasePath) {
			names = append(names, str)
			startPos = append(startPos, off)
		}
		// Add current token to stack.
		stack.Push(tk)
	}

	// Sanity check.
	if len(names) != len(endPos) || len(names) != len(startPos) {
		return nil, errors.New("malformed JSON")
	}

	// Convert offsets to line numbers.
	startLines, err := offsetsToLineNumbers(startPos, content)
	if err != nil {
		return nil, err
	}
	endLines, err := offsetsToLineNumbers(endPos, content)
	if err != nil {
		return nil, err
	}

	// Populate processors.
	for idx, proc := range names {
		processors = append(processors, Processor{
			Type:      proc,
			FirstLine: startLines[idx],
			LastLine:  endLines[idx],
		})
	}
	return processors, nil
}

func nextNewline(r []byte, offset int) int {
	n := len(r)
	if offset >= n {
		return n
	}
	if delta := bytes.IndexByte(r[offset:], '\n'); delta > -1 {
		return offset + delta + 1
	}
	return n
}

func offsetsToLineNumbers(offsets []int, content []byte) (lines []int, err error) {
	if !sort.SliceIsSorted(offsets, func(i, j int) bool {
		return offsets[i] < offsets[j]
	}) {
		return nil, errors.New("input offsets must be sorted")
	}
	lines = make([]int, len(offsets))
	lineEnd := nextNewline(content, 0)
	for i, line := 0, 1; i < len(offsets); {
		if offsets[i] < lineEnd {
			lines[i] = line
			i++
			continue
		}
		for offsets[i] >= lineEnd {
			if lineEnd == len(content) {
				return nil, io.ErrUnexpectedEOF
			}
			line++
			lineEnd = nextNewline(content, lineEnd)
		}
	}
	return lines, nil
}
