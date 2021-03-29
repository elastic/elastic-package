// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"text/template"

	"github.com/pkg/errors"

	_ "github.com/elastic/elastic-package/cmd"
	"github.com/elastic/elastic-package/internal/cobraext"
)

// Generate README
func main() {
	// Load command template
	cmdTmpl, err := template.ParseFiles("./cmd.md.tmpl")
	if err != nil {
		log.Fatal(errors.Wrap(err, "loading command template failed"))
	}

	cmdsDoc := strings.Builder{}
	for cmd, info := range cobraext.CommandInfos {
		fmt.Printf("generating command doc for %s...\n", cmd)
		c := cmdVars{info, cmd}
		if err := cmdTmpl.Execute(&cmdsDoc, c); err != nil {
			log.Fatal(errors.Wrapf(err, "writing documentation for command '%s' failed", cmd))
		}
	}

	// Load README template
	readmeTmpl, err := template.ParseFiles("./readme.md.tmpl")
	if err != nil {
		log.Fatal(errors.Wrap(err, "loading README template failed"))
	}

	readme, err := os.OpenFile("../../README.md", os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		log.Fatal(errors.Wrap(err, "opening README file failed"))
	}
	defer readme.Close()

	r := readmeVars{cmdsDoc.String()}
	if err := readmeTmpl.Execute(readme, r); err != nil {
		log.Fatal(errors.Wrap(err, "writing README failed"))
	}

	fmt.Println("README.md successfully written")
}

type cmdVars struct {
	cobraext.CommandInfo
	Cmd string
}

type readmeVars struct {
	Cmds string
}
