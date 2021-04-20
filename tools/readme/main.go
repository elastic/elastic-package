// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/cmd"
)

// Generate README
func main() {
	commandTemplate := loadCommandTemplate()
	commandsDoc := generateCommandsDoc(commandTemplate)

	readmeTemplate := loadReadmeTemplate()
	generateReadme(readmeTemplate, commandsDoc.String())

	fmt.Println("README.md successfully written")
}

type readmeVars struct {
	Cmds string
}

func loadCommandTemplate() *template.Template {
	cmdTmpl, err := template.ParseFiles("./cmd.md.tmpl")
	if err != nil {
		log.Fatal(errors.Wrap(err, "loading command template failed"))
	}
	return cmdTmpl
}

func generateCommandsDoc(cmdTmpl *template.Template) strings.Builder {
	cmdsDoc := strings.Builder{}
	for _, cmd := range cmd.Commands() {
		log.Printf("generating command doc for %s...\n", cmd.Name())
		if err := cmdTmpl.Execute(&cmdsDoc, cmd); err != nil {
			log.Fatal(errors.Wrapf(err, "writing documentation for command '%s' failed", cmd.Name()))
		}
	}
	return cmdsDoc
}

func loadReadmeTemplate() *template.Template {
	readmeTmpl, err := template.ParseFiles("./readme.md.tmpl")
	if err != nil {
		log.Fatal(errors.Wrap(err, "loading README template failed"))
	}
	return readmeTmpl
}

func generateReadme(readmeTmpl *template.Template, cmdsDoc string) {
	readmePath, err := filepath.Abs("../../README.md")
	if err != nil {
		log.Fatal(errors.Wrap(err, "creating README absolute file path failed"))
	}

	readme, err := os.OpenFile(readmePath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		log.Fatal(errors.Wrapf(err, "opening README file %s failed", readmePath))
	}
	defer readme.Close()

	r := readmeVars{cmdsDoc}
	if err := readmeTmpl.Execute(readme, r); err != nil {
		log.Fatal(errors.Wrapf(err, "writing README file %s failed", readmePath))
	}
}
