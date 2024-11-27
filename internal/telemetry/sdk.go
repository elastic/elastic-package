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
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

func defaultResources(version string) (*resource.Resource, error) {
	return resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.OSName(runtime.GOOS),
			semconv.HostArchKey.String(runtime.GOARCH),
			semconv.ServiceName("elastic-package"),
			semconv.ServiceVersion(version),
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

	// meterProvider, err := newMeterProvider(ctx, res)
	// if err != nil {
	// 	handleErr(err)
	// 	return
	// }

	// shutdownFuncs = append(shutdownFuncs, meterProvider.Shutdown)
	// otel.SetMeterProvider(meterProvider)

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

// func newMeterProvider(ctx context.Context, res *resource.Resource) (*metric.MeterProvider, error) {
// 	metricReader, err := autoexport.NewMetricReader(ctx)
// 	if err != nil {
// 		return nil, err
// 	}
//
// 	meterProvider := metric.NewMeterProvider(
// 		metric.WithResource(res),
// 		metric.WithReader(
// 			metricReader,
// 		),
// 	)
// 	return meterProvider, nil
// }

func newPropagator() propagation.TextMapPropagator {
	return propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	)
}

func newTraceProvider(ctx context.Context, res *resource.Resource) (*trace.TracerProvider, error) {
	traceExporter, err := autoexport.NewSpanExporter(ctx)
	if err != nil {
		return nil, err
	}

	traceProvider := trace.NewTracerProvider(
		trace.WithResource(res),
		trace.WithBatcher(
			traceExporter,
			trace.WithBatchTimeout(time.Second),
		),
	)
	return traceProvider, nil
}
