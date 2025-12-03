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
	subCommandTemplate := loadSubCommandTemplate()
	commandsDoc := generateCommandsDoc(commandTemplate, subCommandTemplate)

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
		log.Fatalf("Loading command template failed: %v", err)
	}
	return cmdTmpl
}

func loadSubCommandTemplate() *template.Template {
	subCmdTmpl, err := template.ParseFiles("./subcmd.md.tmpl")
	if err != nil {
		log.Fatalf("Loading subcommand template failed: %v", err)
	}
	return subCmdTmpl
}

func generateCommandsDoc(cmdTmpl *template.Template, subCommandTemplate *template.Template) strings.Builder {
	cmdsDoc := strings.Builder{}
	for _, cmd := range cmd.Commands() {
		log.Printf("generating command doc for %s...\n", cmd.Name())
		if err := cmdTmpl.Execute(&cmdsDoc, cmd); err != nil {
			log.Fatalf("Writing documentation for command '%s' failed: %v", cmd.Name(), err)
		}
		for _, subCommand := range cmd.Commands() {
			log.Printf("Generating command doc for %s %s...\n", cmd.Name(), subCommand.Name())
			description := subCommand.Long
			if description == "" {
				description = subCommand.Short
			}
			if !strings.HasSuffix(strings.TrimSpace(description), ".") {
				description = description + "."
			}
			templateData := map[string]any{
				"CmdName":     cmd.Name(),
				"SubCmdName":  subCommand.Name(),
				"Context":     cmd.Context(),
				"Description": description,
			}
			if err := subCommandTemplate.Execute(&cmdsDoc, templateData); err != nil {
				log.Fatalf("Writing documentation for command '%s %s' failed: %v", cmd.Name(), subCommand.Name(), err)
			}
		}
	}
	return cmdsDoc
}

func loadReadmeTemplate() *template.Template {
	readmeTmpl, err := template.ParseFiles("./readme.md.tmpl")
	if err != nil {
		log.Fatalf("Loading README template failed: %v", err)
	}
	return readmeTmpl
}

func generateReadme(readmeTmpl *template.Template, cmdsDoc string) {
	readmePath, err := filepath.Abs("../../README.md") //permit:filepath.Abs // Allowing this here as this is a script.
	if err != nil {
		log.Fatalf("Creating README absolute file path failed: %v", err)
	}

	readme, err := os.OpenFile(readmePath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		log.Fatalf("Opening README file %s failed: %v", readmePath, err)
	}
	defer readme.Close()

	r := readmeVars{cmdsDoc}
	if err := readmeTmpl.Execute(readme, r); err != nil {
		log.Fatalf("Writing README file %s failed: %v", readmePath, err)
	}
}
