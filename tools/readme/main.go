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
		log.Fatal(fmt.Errorf("loading command template failed: %s", err))
	}
	return cmdTmpl
}

func generateCommandsDoc(cmdTmpl *template.Template) strings.Builder {
	cmdsDoc := strings.Builder{}
	for _, cmd := range cmd.Commands() {
		log.Printf("generating command doc for %s...\n", cmd.Name())
		if err := cmdTmpl.Execute(&cmdsDoc, cmd); err != nil {
			log.Fatal(fmt.Errorf("writing documentation for command '%s' failed: %s", cmd.Name(), err))
		}
	}
	return cmdsDoc
}

func loadReadmeTemplate() *template.Template {
	readmeTmpl, err := template.ParseFiles("./readme.md.tmpl")
	if err != nil {
		log.Fatal(fmt.Errorf("loading README template failed: %s", err))
	}
	return readmeTmpl
}

func generateReadme(readmeTmpl *template.Template, cmdsDoc string) {
	readmePath, err := filepath.Abs("../../README.md")
	if err != nil {
		log.Fatal(fmt.Errorf("creating README absolute file path failed: %s", err))
	}

	readme, err := os.OpenFile(readmePath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		log.Fatal(fmt.Errorf("opening README file %s failed: %s", readmePath, err))
	}
	defer readme.Close()

	r := readmeVars{cmdsDoc}
	if err := readmeTmpl.Execute(readme, r); err != nil {
		log.Fatal(fmt.Errorf("writing README file %s failed: %sn", readmePath, err))
	}
}
