// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	_ "github.com/mattn/go-sqlite3"

	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/elastic-package/pkg/shell"
)

const (
	createPackagesQuery = "CREATE TABLE `packages` (`id` INTEGER PRIMARY KEY AUTOINCREMENT, `name` VARCHAR(256) NOT NULL, `manifest` TEXT NOT NULL)"
	insertPackageQuery  = "INSERT INTO `packages` (`name`, `manifest`) VALUES (?,?)"
)

var _ shell.Command = &whereCmd{}

type whereCmd struct {
	p                 *Plugin
	name, usage, desc string
}

func registerWhereCmd(p *Plugin) {
	cmd := &whereCmd{
		p:     p,
		name:  "where",
		usage: `where "query"`,
		desc:  "Select a list of packages based on some conditions. Reads from context 'Shell.DB' and updates context 'Shell.Packages'.",
	}
	p.RegisterCommand(cmd)
}

func (c *whereCmd) Name() string  { return c.name }
func (c *whereCmd) Usage() string { return c.usage }
func (c *whereCmd) Desc() string  { return c.desc }

func (c *whereCmd) Exec(wd string, args []string, _, stderr io.Writer) error {
	db, ok := c.p.GetValueFromCtx(ctxKeyDB).(*sql.DB)
	if !ok {
		return errors.New("db connection not found in context")
	}
	conditions := strings.Join(args, " ")
	query := `SELECT name FROM packages`
	if conditions != "" {
		query = fmt.Sprintf("%s WHERE %s", query, conditions)
	}

	rows, err := db.Query(query)
	if err != nil {
		return err
	}
	defer rows.Close()

	var pkgs []string
	var pkg string
	for rows.Next() {
		if err := rows.Scan(&pkg); err != nil {
			return err
		}
		pkgs = append(pkgs, pkg)
	}

	c.p.AddValueToCtx(ctxKeyPackages, pkgs)
	fmt.Fprintf(stderr, "Found %d packages\n", len(pkgs))
	return nil
}

var _ shell.Command = &initdbCmd{}

type initdbCmd struct {
	p                 *Plugin
	name, usage, desc string
}

func registerInitdbCmd(p *Plugin) {
	cmd := &initdbCmd{
		p:     p,
		name:  "initdb",
		usage: "initdb",
		desc:  "Initializes the packages database. Sets context 'Shell.DB'.",
	}
	p.RegisterCommand(cmd)
}

func (c *initdbCmd) Name() string  { return c.name }
func (c *initdbCmd) Usage() string { return c.usage }
func (c *initdbCmd) Desc() string  { return c.desc }

func (c *initdbCmd) Exec(wd string, args []string, _, stderr io.Writer) error {
	fmt.Fprintln(stderr, "Initializing database...")

	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		return err
	}

	if _, err := db.Exec(createPackagesQuery); err != nil {
		return err
	}

	packagesPath := filepath.Join(wd, "packages")
	if _, err := os.Stat(packagesPath); err != nil {
		return err
	}

	entries, err := os.ReadDir(packagesPath)
	if err != nil {
		return err
	}

	var count int
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		root := filepath.Join(packagesPath, e.Name())
		manifest, err := packages.ReadPackageManifestFromPackageRoot(root)
		if err != nil {
			return err
		}
		p, err := json.Marshal(manifest)
		if err != nil {
			return err
		}
		if _, err := db.Exec(insertPackageQuery, e.Name(), string(p)); err != nil {
			return err
		}
		count++
	}

	fmt.Fprintf(stderr, "Loaded %d packages\n", count)

	c.p.AddValueToCtx(ctxKeyDB, db)

	return nil
}
