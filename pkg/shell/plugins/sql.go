// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	_ "github.com/mattn/go-sqlite3"
	"github.com/spf13/pflag"

	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/elastic-package/pkg/shell"
)

const (
	createPackagesQuery = "CREATE TABLE `packages` (`id` INTEGER PRIMARY KEY AUTOINCREMENT, `name` VARCHAR(256) NOT NULL, `manifest` TEXT NOT NULL)"
	insertPackageQuery  = "INSERT INTO `packages` (`name`, `manifest`) VALUES (?,?)"
)

var _ shell.Command = whereCmd{}

type whereCmd struct{}

func (whereCmd) Usage() string {
	return `where "query"`
}

func (whereCmd) Desc() string {
	return "Select a list of packages based on some conditions. Reads from context 'Shell.DB' and updates context 'Shell.Packages'."
}

func (whereCmd) Flags() *pflag.FlagSet {
	return nil
}

func (whereCmd) Exec(ctx context.Context, flags *pflag.FlagSet, args []string, _, stderr io.Writer) (context.Context, error) {
	db, ok := ctx.Value(ctxKeyDB).(*sql.DB)
	if !ok {
		return ctx, errors.New("db connection not found in context")
	}
	conditions := strings.Join(args, " ")
	query := `SELECT name FROM packages`
	if conditions != "" {
		query = fmt.Sprintf("%s WHERE %s", query, conditions)
	}

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pkgs []string
	var pkg string
	for rows.Next() {
		if err := rows.Scan(&pkg); err != nil {
			return nil, err
		}
		pkgs = append(pkgs, pkg)
	}

	ctx = context.WithValue(ctx, ctxKeyPackages, pkgs)
	fmt.Fprintf(stderr, "Found %d packages\n", len(pkgs))
	return ctx, nil
}

var _ shell.Command = initdbCmd{}

type initdbCmd struct{}

func (initdbCmd) Usage() string {
	return "initdb"
}

func (initdbCmd) Desc() string {
	return "Initializes the packages database. Sets context 'Shell.DB'."
}

func (initdbCmd) Flags() *pflag.FlagSet {
	return nil
}

func (initdbCmd) Exec(ctx context.Context, flags *pflag.FlagSet, args []string, _, stderr io.Writer) (context.Context, error) {
	fmt.Fprintln(stderr, "Initializing database...")

	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		return ctx, err
	}

	if _, err := db.Exec(createPackagesQuery); err != nil {
		return ctx, err
	}

	packagesPath := filepath.Join(".", "packages")
	if _, err := os.Stat(packagesPath); err != nil {
		return ctx, err
	}

	entries, err := os.ReadDir(packagesPath)
	if err != nil {
		return ctx, err
	}

	var c int
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		root := filepath.Join(packagesPath, e.Name())
		manifest, err := packages.ReadPackageManifestFromPackageRoot(root)
		if err != nil {
			return ctx, err
		}
		p, err := json.Marshal(manifest)
		if err != nil {
			return ctx, err
		}
		if _, err := db.Exec(insertPackageQuery, e.Name(), string(p)); err != nil {
			return ctx, err
		}
		c++
	}

	fmt.Fprintf(stderr, "Loaded %d packages\n", c)

	ctx = context.WithValue(ctx, ctxKeyDB, db)

	return ctx, nil
}
