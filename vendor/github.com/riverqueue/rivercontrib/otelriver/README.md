# otelriver [![Build Status](https://github.com/riverqueue/rivercontrib/actions/workflows/ci.yaml/badge.svg?branch=master)](https://github.com/riverqueue/rivercontrib/actions) [![Go Reference](https://pkg.go.dev/badge/github.com/riverqueue/rivercontrib.svg)](https://pkg.go.dev/github.com/riverqueue/rivercontrib/otelriver)

[OpenTelemetry](https://opentelemetry.io/) utilities for the [River job queue](https://github.com/riverqueue/river).

See [`example_middleware_test.go`](./example_middleware_test.go) for usage details.

## Options

The middleware supports these options:

``` go
middleware := otelriver.NewMiddleware(&MiddlewareConfig{
    DurationUnit:                "ms",
    EnableSemanticMetrics:       true,
    EnableWorkSpanJobKindSuffix: true,
    MeterProvider:               meterProvider,
    TracerProvider:              tracerProvider,
})
```

* `DurationUnit`: The unit which durations are emitted as, either "ms" (milliseconds) or "s" (seconds). Defaults to seconds.
* `EnableSemanticMetrics`: Causes the middleware to emit metrics compliant with OpenTelemetry's ["semantic conventions"](https://opentelemetry.io/docs/specs/semconv/messaging/messaging-metrics/) for message clients. This has the effect of having all messaging systems share the same common metric names, with attributes differentiating them.
* `EnableWorkSpanJobKindSuffix`: Appends the job kind a suffix to work spans so they look like `river.work/my_job` instead of `river.work`.
* `MeterProvider`: Injected OpenTelemetry meter provider. The global meter provider is used by default.
* `TracerProvider`: Injected OpenTelemetry tracer provider. The global tracer provider is used by default.

## Use with DataDog

See [using the OpenTelemetry API with DataDog](https://docs.datadoghq.com/tracing/trace_collection/custom_instrumentation/go/otel/) and the examples in [`datadogriver`](../datadogriver/) for how to configure a DataDog OpenTelemetry tracer provider.
