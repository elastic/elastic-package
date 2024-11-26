// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package telemetry

import (
	"context"
	"strings"

	"github.com/spf13/cobra"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

const (
	AttributeKeyCobraCommand     attribute.Key = "cobra.command"
	AttributeKeyCobraSubcommand  attribute.Key = "cobra.subcommand"
	AttributeKeyCobraCommandPath attribute.Key = "cobra.commandpath"

	AttributeKeyPackageName        attribute.Key = "package.name"
	AttributeKeyPackageVersion     attribute.Key = "package.version"
	AttributeKeyPackageSpecVersion attribute.Key = "package.specversion"
)

func StartSpanForCommand(tracer trace.Tracer, cmd *cobra.Command) (context.Context, trace.Span) {
	// via https://stackoverflow.com/a/78486358/2257038
	commandParts := strings.Fields(cmd.CommandPath())
	command := commandParts[0]
	subcommand := []string{}
	if len(commandParts) > 1 {
		subcommand = commandParts[1:]
	}

	ctx, span := tracer.Start(
		cmd.Context(),
		cmd.CommandPath(),
		trace.WithAttributes(
			AttributeKeyCobraCommand.String(command),
			AttributeKeyCobraSubcommand.StringSlice(subcommand),
			AttributeKeyCobraCommandPath.String(cmd.CommandPath()),
		))

	cmd.SetContext(ctx)

	return ctx, span
}
