// Package instr provides instrumentation for observability pipelines.
package instr

import (
	"context"
	"errors"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func SetupOTel(
	ctx context.Context,
	otlpEndpoint string,
	serviceName string,
) (shutdown func(context.Context) error, reterr error) {
	var shutdownFuncs []func(context.Context) error
	shutdownFunc := func(ctx context.Context) error {
		var err error
		for _, fn := range shutdownFuncs {
			err = errors.Join(err, fn(ctx))
		}
		shutdownFuncs = nil
		return err
	}

	defer func() {
		if reterr == nil {
			return
		}
		err := shutdownFunc(ctx)
		reterr = errors.Join(err, reterr)
	}()

	prop := propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	)

	res, err := resource.New(ctx, resource.WithAttributes(semconv.ServiceNameKey.String(serviceName)))
	if err != nil {
		return nil, fmt.Errorf("resource: %w", err)
	}

	otel.SetTextMapPropagator(prop)

	conn, err := grpc.NewClient(otlpEndpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("grpc: %w", err)
	}

	traceExporter, err := otlptracegrpc.New(ctx, otlptracegrpc.WithGRPCConn(conn))
	if err != nil {
		return nil, fmt.Errorf("trace exporter: %w", err)
	}

	tracerProvider := trace.NewTracerProvider(
		trace.WithSampler(trace.AlwaysSample()),
		trace.WithSpanProcessor(trace.NewBatchSpanProcessor(traceExporter)),
		trace.WithResource(res),
	)

	otel.SetTracerProvider(tracerProvider)

	// TODO: add metrics

	return shutdownFunc, err
}
