// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package pipeline

import (
	"bytes"
	"encoding/json"
	"io"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
)

type Processor struct {
	Type        string `yaml:"-"`
	Tag         string
	Description string
	Line        int `yaml:"-"`
}

func (p Resource) Processors() (procs []Processor, err error) {
	switch p.Format {
	case "yaml", "yml":
		procs, err = ProcessorsFromYAMLPipeline(p.Content)
	case "json":
		procs, err = ProcessorsFromJSONPipeline(p.Content)
	default:
		return nil, errors.Errorf("unsupported pipeline Format: %s", p.Format)
	}
	return procs, errors.Wrapf(err, "failure processing %s pipeline '%s'", p.Format, p.FileName())
}

func ProcessorsFromYAMLPipeline(content []byte) (procs []Processor, err error) {
	type pipeline struct {
		Processors []yaml.Node
	}
	var p pipeline
	if err = yaml.Unmarshal(content, &p); err != nil {
		return nil, err
	}
	for idx, entry := range p.Processors {
		if entry.Kind != yaml.MappingNode || len(entry.Content) != 2 {
			return nil, errors.Errorf("processor#%d is not a single key map (kind:%v Content:%d)", idx, entry.Kind, len(entry.Content))
		}
		var proc Processor
		if err := entry.Content[1].Decode(&proc); err != nil {
			return nil, errors.Wrapf(err, "error decoding processor#%d configuration", idx)
		}
		if err := entry.Content[0].Decode(&proc.Type); err != nil {
			return nil, errors.Wrapf(err, "error decoding processor#%d type", idx)
		}
		proc.Line = entry.Line
		procs = append(procs, proc)
	}
	return procs, nil
}

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
	return io.EOF // ???
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

var processorJSONPath = tokenStack{json.Delim('{'), "processors", json.Delim('['), json.Delim('{')}

func ProcessorsFromJSONPipeline(content []byte) (procs []Processor, err error) {
	var processors []string
	var offsets []int
	decoder := json.NewDecoder(bytes.NewReader(content))
	var stack tokenStack

	for {
		off := int(decoder.InputOffset())
		tk, err := decoder.Token()
		if err == io.EOF {
			break
		}
		delim, isDelim := tk.(json.Delim)
		if isDelim && (delim == '}' || delim == ']') {
			stack.PopUntil(delim - 2) // `}`-2 = `{` and `]`-2 = `[`
			if stack.TopIsString() {
				stack.Pop()
			}
			continue
		}
		if !isDelim && stack.TopIsString() {
			stack.Pop()
			continue
		}
		if str, ok := tk.(string); ok && stack.Equals(processorJSONPath) {
			processors = append(processors, str)
			offsets = append(offsets, off)
		}
		stack.Push(tk)
	}
	lines, err := offsetsToLineNumbers(offsets, content)
	if err != nil {
		return nil, err
	}

	procs = make([]Processor, len(processors))
	for idx, proc := range processors {
		procs[idx] = Processor{
			Type: proc,
			Line: lines[idx],
		}
	}
	return procs, nil
}

func offsetsToLineNumbers(offsets []int, content []byte) (lines []int, err error) {
	nextNewline := func(r []byte, offset int) int {
		n := len(r)
		if offset >= n {
			return n
		}
		if delta := bytes.IndexByte(r[offset+1:], '\n'); delta > -1 {
			return offset + delta + 1
		}
		return n
	}
	lineEnd := nextNewline(content, -1)
	line := 1
	lines = make([]int, len(offsets))
	for i := 0; i < len(offsets); {
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
