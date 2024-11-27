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
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

var (
	CmdTracer trace.Tracer
	CmdMeter  metric.Meter

	ProfilesListSuccessCnt metric.Int64Counter
	ProfilesListFailureCnt metric.Int64Counter
)

func defaultResources(version string) (*resource.Resource, error) {
	extraResources, err := resource.New(
		context.Background(),
		resource.WithOSType(),
		resource.WithHostID(),
	)
	if err != nil {
		return nil, err
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
			semconv.DeploymentEnvironment("development"),
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

	meterProvider, err := newMeterProvider(ctx, res)
	if err != nil {
		handleErr(err)
		return
	}

	shutdownFuncs = append(shutdownFuncs, meterProvider.Shutdown)
	otel.SetMeterProvider(meterProvider)

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

func newMeterProvider(ctx context.Context, res *resource.Resource) (*sdkmetric.MeterProvider, error) {
	metricReader, err := autoexport.NewMetricReader(ctx)
	if err != nil {
		return nil, err
	}

	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(
			metricReader,
		),
	)
	return meterProvider, nil
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

func SetupMetrics(meter metric.Meter) {
	var err error
	ProfilesListSuccessCnt, err = meter.Int64Counter("elastic-package.profiles.list.success",
		metric.WithDescription("The number of executions of profiles list finished successfully"),
		metric.WithUnit("{profile}"))
	if err != nil {
		panic(err)
	}
	ProfilesListFailureCnt, err = meter.Int64Counter("elastic-package.profiles.list.failure",
		metric.WithDescription("The number of executions of profiles list finished with failure"),
		metric.WithUnit("{profile}"))
	if err != nil {
		panic(err)
	}

	return
}
