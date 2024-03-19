// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package common

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

var jsonUnmarshalUsingNumberTests = []struct {
	name string
	msg  string
}{
	{
		name: "empty",
		msg:  "", // Will error "unexpected end of JSON input".
	},
	{
		name: "string",
		msg:  `"message"`,
	},
	{
		name: "array",
		msg:  "[1,2,3,4,5]",
	},
	{
		name: "object",
		msg:  `{"key":42}`,
	},
	{
		name: "object",
		msg:  `{"key":42}answer`, // Will error "invalid character 'a' after top-level value".
	},
	// Test extra data whitespace parity with json.Unmarshal for error parity.
	{
		name: "object",
		msg:  `{"key":42} `,
	},
	{
		name: "object",
		msg:  `{"key":42}` + "\t",
	},
	{
		name: "object",
		msg:  `{"key":42}` + "\r",
	},
	{
		name: "object",
		msg:  `{"key":42}` + "\n",
	},
	{
		name: "0x1p52+1",
		msg:  fmt.Sprint(uint64(0x1p52) + 1),
	},
	{
		name: "0x1p53-1",
		msg:  fmt.Sprint(uint64(0x1p53) - 1),
	},
	// The following three cases will fail if json.Unmarshal is used in place
	// of jsonUnmarshalUsingNumber, as they are past the cutover.
	{
		name: "0x1p53+1",
		msg:  fmt.Sprint(uint64(0x1p53) + 1),
	},
	{
		name: "0x1p54+1",
		msg:  fmt.Sprint(uint64(0x1p54) + 1),
	},
	{
		name: "long",
		msg:  "9223372036854773807",
	},
}

func TestJsonUnmarshalUsingNumberRoundTrip(t *testing.T) {
	// This tests that jsonUnmarshalUsingNumber behaves the same
	// way as json.Unmarshal with the exception that numbers are
	// not unmarshaled through float64. This is important to avoid
	// low-bit truncation of long numeric values that are greater
	// than or equal to 0x1p53, the limit of bijective equivalence
	// with 64 bit-integers.

	for _, test := range jsonUnmarshalUsingNumberTests {
		t.Run(test.name, func(t *testing.T) {
			var val interface{}
			err := JSONUnmarshalUsingNumber([]byte(test.msg), &val)

			// Confirm that we get the same errors with jsonUnmarshalUsingNumber
			// as are returned by json.Unmarshal.
			jerr := json.Unmarshal([]byte(test.msg), new(interface{}))
			if (err == nil) != (jerr == nil) {
				t.Errorf("unexpected error: got:%#v want:%#v", err, jerr)
			}
			if err != nil {
				return
			}

			// Confirm that we round-trip the message correctly without
			// alteration beyond trailing whitespace.
			got, err := json.Marshal(val)
			if err != nil {
				t.Errorf("unexpected error: got:%#v want:%#v", err, jerr)
			}
			// Truncate trailing whitespace from the input since it won't
			// be rendered in the output. This set of space characters is
			// defined in encoding/json/scanner.go as func isSpace.
			want := strings.TrimRight(test.msg, " \t\r\n")
			if string(got) != want {
				t.Errorf("unexpected result: got:%v want:%v", val, want)
			}
		})
	}
}
