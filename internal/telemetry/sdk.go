// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package telemetry

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"time"

	"go.opentelemetry.io/contrib/exporters/autoexport"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

var (
	CmdTracer trace.Tracer
	CmdMeter  metric.Meter
)

func defaultResources(version string) (*resource.Resource, error) {
	ctx := context.Background()
	extraResources, err := resource.New(
		ctx,
		resource.WithOSType(),
	)
	if err != nil {
		return nil, err
	}

	var hostResources, containerResources *resource.Resource
	hostResources, err = resource.New(
		ctx,
		resource.WithHostID(),
	)
	if err == nil {
		// (all?) container hosts cannot retrieve the host id
		// failed to set up default Resource configuration: error detecting resource: host id not found in: /etc/machine-id or /var/lib/dbus/machine-id
		extraResources, err = resource.Merge(extraResources, hostResources)
		if err != nil {
			return nil, err
		}
	}
	containerResources, err = resource.New(
		ctx,
		resource.WithContainerID(),
	)
	if err == nil {
		extraResources, err = resource.Merge(extraResources, containerResources)
		if err != nil {
			return nil, err
		}
	}

	defaultResource, err := resource.Merge(resource.Default(), extraResources)
	if err != nil {
		return nil, err
	}

	return resource.Merge(
		defaultResource,
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.HostArchKey.String(runtime.GOARCH),
			semconv.ServiceName("elastic-package"),
			semconv.ServiceVersion(version),
			semconv.DeploymentEnvironment("dev"),
		),
	)
}

func SetupOTelSDK(ctx context.Context, version string) (shutdown func(context.Context) error, err error) {
	var shutdownFuncs []func(context.Context) error
	shutdown = func(c context.Context) error {
		var err error
		for _, fn := range shutdownFuncs {
			err = errors.Join(err, fn(ctx))
		}
		shutdownFuncs = nil
		return err
	}
	handleErr := func(inErr error) {
		err = errors.Join(inErr, shutdown(ctx))
	}

	res, err := defaultResources(version)
	if err != nil {
		return nil, fmt.Errorf("failed to set up default Resource configuration: %w", err)
	}

	// TODO: Check if this is actually needed
	prop := newPropagator()
	otel.SetTextMapPropagator(prop)

	tracerProvider, err := newTraceProvider(ctx, res)
	if err != nil {
		handleErr(err)
		return
	}
	shutdownFuncs = append(shutdownFuncs, tracerProvider.Shutdown)
	otel.SetTracerProvider(tracerProvider)

	return
}

func newPropagator() propagation.TextMapPropagator {
	return propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	)
}

func newTraceProvider(ctx context.Context, res *resource.Resource) (*sdktrace.TracerProvider, error) {
	traceExporter, err := autoexport.NewSpanExporter(ctx)
	if err != nil {
		return nil, err
	}

	traceProvider := sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithBatcher(
			traceExporter,
			sdktrace.WithBatchTimeout(time.Second),
		),
	)
	return traceProvider, nil
}
