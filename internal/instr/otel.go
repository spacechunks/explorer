// Package instr provides instrumentation for observability pipelines.
package instr

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	oteltrace "go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

func SetupOTel(
	ctx context.Context,
	serviceName string,
	disable bool,
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

	traceExporter, err := otlptracegrpc.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("trace exporter: %w", err)
	}

	tracerProvider := trace.NewTracerProvider(
		trace.WithSampler(trace.AlwaysSample()),
		trace.WithSpanProcessor(trace.NewBatchSpanProcessor(traceExporter)),
		trace.WithResource(res),
	)

	otel.SetTracerProvider(tracerProvider)

	if disable {
		otel.SetTracerProvider(noop.NewTracerProvider())
	}

	// TODO: add metrics

	return shutdownFunc, err
}

type OTelSlogHandler struct {
	slog.Handler
}

func (h OTelSlogHandler) Handle(ctx context.Context, r slog.Record) error {
	span := oteltrace.SpanFromContext(ctx)
	if !span.SpanContext().IsValid() {
		return h.Handler.Handle(ctx, r)
	}

	r.AddAttrs(
		slog.String("trace_id", span.SpanContext().TraceID().String()),
		slog.String("span_id", span.SpanContext().SpanID().String()),
	)

	return h.Handler.Handle(ctx, r)
}
