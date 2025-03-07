// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package main

import (
	"flag"
	"fmt"
	"os"
)

// Tool based on the code at https://github.com/maxmind/MaxMind-DB
// (commit: https://github.com/maxmind/MaxMind-DB/commit/0ec71808b19669e9e1bf5e63a8c83b202d9bd115)

func main() {
	source := flag.String("source", "", "Source data directory")
	target := flag.String("target", "", "Destination directory for the generated mmdb files")

	flag.Parse()

	w, err := newWriter(*source, *target)
	if err != nil {
		fmt.Printf("creating writer: %+v\n", err)
		os.Exit(1)
	}

	if err := w.WriteGeoIP2TestDB(); err != nil {
		fmt.Printf("writing GeoIP2 test databases: %+v\n", err)
		os.Exit(1)
	}
}
