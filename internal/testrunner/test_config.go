// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package testrunner

import (
	"fmt"
	"net/url"
)

// SkipConfig allows a test to be marked as skipped
type SkipConfig struct {
	// Reason is the short reason for why this test should be skipped.
	Reason string `config:"reason"`

	// Link is a URL where more details about the skipped test can be found.
	Link packedURL `config:"link"`
}

type packedURL struct {
	*url.URL
}

func (u *packedURL) Unpack(s string) error {
	var err error
	u.URL, err = url.Parse(s)
	if err != nil {
		return err
	}
	return nil
}

func (s SkipConfig) String() string {
	return fmt.Sprintf("%s [%s]", s.Reason, s.Link)
}

// SkippableConfig is a test configuration that allows skipping. This
// struct is intended for embedding in concrete test configuration structs.
type SkippableConfig struct {
	// Skip allows this test to be skipped.
	Skip *SkipConfig `config:"skip"`
}
