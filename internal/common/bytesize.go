// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package common

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strconv"

	"gopkg.in/yaml.v3"
)

// Common units for sizes in bytes.
const (
	Byte     = ByteSize(1)
	KiloByte = 1024 * Byte
	MegaByte = 1024 * KiloByte
	GigaByte = 1024 * MegaByte
)

const (
	byteString     = "B"
	kiloByteString = "KB"
	megaByteString = "MB"
	gigaByteString = "GB"
)

// ByteSize represents the size of a file.
type ByteSize uint64

// Ensure FileSize implements these interfaces.
var (
	_ json.Marshaler   = new(ByteSize)
	_ json.Unmarshaler = new(ByteSize)
	_ yaml.Marshaler   = new(ByteSize)
	_ yaml.Unmarshaler = new(ByteSize)
)

func parseFileSizeInt(s string) (uint64, error) {
	// os.FileInfo reports size as int64, don't support bigger numbers.
	maxBitSize := 63
	return strconv.ParseUint(s, 10, maxBitSize)
}

// MarshalJSON implements the json.Marshaler interface for FileSize, it returns
// the string representation in a format that can be unmarshaled back to an
// equivalent value.
func (s ByteSize) MarshalJSON() ([]byte, error) {
	return []byte(`"` + s.String() + `"`), nil
}

// MarshalYAML implements the yaml.Marshaler interface for FileSize, it returns
// the string representation in a format that can be unmarshaled back to an
// equivalent value.
func (s ByteSize) MarshalYAML() (interface{}, error) {
	return s.String(), nil
}

// UnmarshalJSON implements the json.Unmarshaler interface for FileSize.
func (s *ByteSize) UnmarshalJSON(d []byte) error {
	// Support unquoted plain numbers.
	bytes, err := parseFileSizeInt(string(d))
	if err == nil {
		*s = ByteSize(bytes)
		return nil
	}

	var text string
	err = json.Unmarshal(d, &text)
	if err != nil {
		return err
	}

	return s.unmarshalString(text)
}

// UnmarshalYAML implements the yaml.Unmarshaler interface for FileSize.
func (s *ByteSize) UnmarshalYAML(value *yaml.Node) error {
	// Support unquoted plain numbers.
	bytes, err := parseFileSizeInt(value.Value)
	if err == nil {
		*s = ByteSize(bytes)
		return nil
	}

	return s.unmarshalString(value.Value)
}

var bytesPattern = regexp.MustCompile(fmt.Sprintf(`^(\d+(\.\d+)?)(%s|%s|%s|%s|)$`, byteString, kiloByteString, megaByteString, gigaByteString))

func (s *ByteSize) unmarshalString(text string) error {
	match := bytesPattern.FindStringSubmatch(text)
	if len(match) < 3 {
		return fmt.Errorf("invalid format for size in bytes (%s)", text)
	}

	if match[2] == "" {
		q, err := parseFileSizeInt(match[1])
		if err != nil {
			return fmt.Errorf("invalid format for size in bytes (%s): %w", text, err)
		}

		unit := match[3]
		switch unit {
		case gigaByteString:
			*s = ByteSize(q) * GigaByte
		case megaByteString:
			*s = ByteSize(q) * MegaByte
		case kiloByteString:
			*s = ByteSize(q) * KiloByte
		case byteString, "":
			*s = ByteSize(q) * Byte
		default:
			return fmt.Errorf("invalid unit for filesize (%s): %s", text, unit)
		}
	} else {
		q, err := strconv.ParseFloat(match[1], 64)
		if err != nil {
			return fmt.Errorf("invalid format for size in bytes (%s): %w", text, err)
		}

		unit := match[3]
		switch unit {
		case gigaByteString:
			*s = approxFloat(q, GigaByte)
		case megaByteString:
			*s = approxFloat(q, MegaByte)
		case kiloByteString:
			*s = approxFloat(q, KiloByte)
		case byteString, "":
			*s = approxFloat(q, Byte)
		default:
			return fmt.Errorf("invalid unit for filesize (%s): %s", text, unit)
		}
	}

	return nil
}

func approxFloat(n float64, unit ByteSize) ByteSize {
	approx := n * float64(unit)
	return ByteSize(math.Round(approx))
}

// String returns the string representation of the FileSize.
func (s ByteSize) String() string {
	format := func(q ByteSize, unit string) string {
		return fmt.Sprintf("%d%s", q, unit)
	}

	if s >= GigaByte && (s%GigaByte == 0) {
		return format(s/GigaByte, gigaByteString)
	}

	if s >= MegaByte && (s%MegaByte == 0) {
		return format(s/MegaByte, megaByteString)
	}

	if s >= KiloByte && (s%KiloByte == 0) {
		return format(s/KiloByte, kiloByteString)
	}

	return format(s, byteString)
}
