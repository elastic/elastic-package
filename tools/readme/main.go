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
	if err := generateReadme(readmeTemplate, commandsDoc.String()); err != nil {
		log.Fatalf("generating README: %v", err)
	}

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

func generateReadme(readmeTmpl *template.Template, cmdsDoc string) (err error) {
	readmePath, err := filepath.Abs("../../README.md")
	if err != nil {
		return fmt.Errorf("creating README absolute file path: %w", err)
	}

	readme, err := os.OpenFile(readmePath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("opening README file %s: %w", readmePath, err)
	}
	defer func() {
		if cerr := readme.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	r := readmeVars{cmdsDoc}
	if err := readmeTmpl.Execute(readme, r); err != nil {
		return fmt.Errorf("writing README file %s: %w", readmePath, err)
	}
	return nil
}
