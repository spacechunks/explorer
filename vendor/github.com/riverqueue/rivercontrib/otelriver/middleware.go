package otelriver

import (
	"cmp"
	"context"
	"errors"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
)

const (
	// OpenTelemetry docs recommended this be a fully qualified Go package name.
	name = "github.com/riverqueue/rivercontrib/otelriver"

	// Prefix added to than names of all emitted metrics and traces.
	prefix = "river."
)

// MiddlewareConfig is configuration for River's OpenTelemetry middleware.
type MiddlewareConfig struct {
	// DurationUnit selects the unit in which duration metrics like
	// `river.work_duration` are emitted.
	//
	// Must be one of "ms" (milliseconds) or "s" (seconds). Defaults to seconds.
	//
	// Does not modify metrics emitted by EnableSemanticMetrics because those
	// are constrained to seconds by specification.
	DurationUnit string

	// EnableSemanticMetrics emits metrics compliant with OpenTelemetry's
	// "semantic conventions" for messaging clients:
	//
	// https://opentelemetry.io/docs/specs/semconv/messaging/messaging-metrics/
	//
	// This has the effect of having all messaging systems share the same common
	// metric names, with attributes differentiating them.
	EnableSemanticMetrics bool

	// EnableWorkSpanJobKindSuffix appends the job kind a suffix to work spans
	// so they look like `river.work/my_job` instead of `river.work`.
	EnableWorkSpanJobKindSuffix bool

	// MeterProvider is a MeterProvider to base metrics on. May be left as nil
	// to use the default global provider.
	MeterProvider metric.MeterProvider

	// TracerProvider is a TracerProvider to base traces on. May be left as nil
	// to use the default global provider.
	TracerProvider trace.TracerProvider
}

// Middleware is a River middleware that emits OpenTelemetry metrics when jobs
// are inserted or worked.
type Middleware struct {
	river.MiddlewareDefaults

	config  *MiddlewareConfig
	meter   metric.Meter
	metrics middlewareMetrics
	tracer  trace.Tracer
}

// Bundle of metrics associated with a middleware.
type middlewareMetrics struct {
	insertCount                      metric.Int64Counter
	insertManyCount                  metric.Int64Counter
	insertManyDuration               metric.Float64Gauge
	insertManyDurationHistogram      metric.Float64Histogram
	messagingClientConsumedMessages  metric.Int64Counter
	messagingClientOperationDuration metric.Float64Histogram
	messagingClientSentMessages      metric.Int64Counter
	messagingProcessDuration         metric.Float64Histogram
	workCount                        metric.Int64Counter
	workDuration                     metric.Float64Gauge
	workDurationHistogram            metric.Float64Histogram
}

// NewMiddleware initializes a new River OpenTelemetry middleware.
//
// config may be nil.
func NewMiddleware(config *MiddlewareConfig) *Middleware {
	if config == nil {
		config = &MiddlewareConfig{}
	}

	durationUnit := cmp.Or(config.DurationUnit, "s")
	if durationUnit != "ms" && durationUnit != "s" {
		panic("duration unit must be one of ms or s")
	}

	meterProvider := otel.GetMeterProvider()
	if config.MeterProvider != nil {
		meterProvider = config.MeterProvider
	}

	tracerProvider := otel.GetTracerProvider()
	if config.TracerProvider != nil {
		tracerProvider = config.TracerProvider
	}

	meter := meterProvider.Meter(name)

	metrics := middlewareMetrics{
		// See unit guidelines:
		//
		// https://opentelemetry.io/docs/specs/semconv/general/metrics/#instrument-units
		insertCount:                 mustInt64Counter(meter, prefix+"insert_count", metric.WithDescription("Number of jobs inserted"), metric.WithUnit("{job}")),
		insertManyCount:             mustInt64Counter(meter, prefix+"insert_many_count", metric.WithDescription("Number of job batches inserted (all jobs are inserted in a batch, but batches may be one job)"), metric.WithUnit("{job_batch}")),
		insertManyDuration:          mustFloat64Gauge(meter, prefix+"insert_many_duration", metric.WithDescription("Duration of job batch insertion"), metric.WithUnit(durationUnit)),
		insertManyDurationHistogram: mustFloat64Histogram(meter, prefix+"insert_many_duration_histogram", metric.WithDescription("Duration of job batch insertion (histogram)"), metric.WithUnit(durationUnit)),
		workCount:                   mustInt64Counter(meter, prefix+"work_count", metric.WithDescription("Number of jobs worked"), metric.WithUnit("{job}")),
		workDuration:                mustFloat64Gauge(meter, prefix+"work_duration", metric.WithDescription("Duration of job being worked"), metric.WithUnit(durationUnit)),
		workDurationHistogram:       mustFloat64Histogram(meter, prefix+"work_duration_histogram", metric.WithDescription("Duration of job being worked (histogram)"), metric.WithUnit(durationUnit)),
	}

	if config.EnableSemanticMetrics {
		metrics.messagingClientConsumedMessages = mustInt64Counter(meter, "messaging.client.consumed.messages", metric.WithDescription("Number of messages that were delivered to the application."))
		metrics.messagingClientOperationDuration = mustFloat64Histogram(meter, "messaging.client.operation.duration", metric.WithDescription("Duration of messaging operation initiated by a producer or consumer client."), metric.WithUnit(durationUnit))
		metrics.messagingClientSentMessages = mustInt64Counter(meter, "messaging.client.sent.messages", metric.WithDescription("Number of messages producer attempted to send to the broker."))
		metrics.messagingProcessDuration = mustFloat64Histogram(meter, "messaging.process.duration", metric.WithDescription("Duration of processing operation."), metric.WithUnit(durationUnit))
	}

	return &Middleware{
		config:  config,
		meter:   meter,
		metrics: metrics,
		tracer:  tracerProvider.Tracer(name),
	}
}

func (m *Middleware) InsertMany(ctx context.Context, manyParams []*rivertype.JobInsertParams, doInner func(ctx context.Context) ([]*rivertype.JobInsertResult, error)) ([]*rivertype.JobInsertResult, error) {
	ctx, span := m.tracer.Start(ctx, prefix+"insert_many",
		trace.WithSpanKind(trace.SpanKindProducer))
	defer span.End()

	attrs := []attribute.KeyValue{
		attribute.String("status", ""), // replaced below
	}
	const statusIndex = 0

	var (
		begin     = time.Now()
		err       error
		insertRes []*rivertype.JobInsertResult
		panicked  = true // set to false if program leaves normally
	)
	defer func() {
		duration := m.durationInPreferredUnit(time.Since(begin))

		setStatus(attrs, statusIndex, span, panicked, err)
		span.SetAttributes(attrs...) // set after finalizing status

		// This allocates a new slice, so make sure to do it as few times as possible.
		measurementOpt := metric.WithAttributes(attrs...)

		m.metrics.insertCount.Add(ctx, int64(len(manyParams)), measurementOpt)
		m.metrics.insertManyCount.Add(ctx, 1, measurementOpt)
		m.metrics.insertManyDuration.Record(ctx, duration, measurementOpt)
		m.metrics.insertManyDurationHistogram.Record(ctx, duration, measurementOpt)

		if m.config.EnableSemanticMetrics {
			measurementOpt := metric.WithAttributes(
				attribute.String("messaging.operation.name", "insert_many"),
				attribute.String("messaging.system", "river"),
			)

			m.metrics.messagingClientOperationDuration.Record(ctx, duration, measurementOpt)
			m.metrics.messagingClientSentMessages.Add(ctx, int64(len(manyParams)), measurementOpt)
		}
	}()

	insertRes, err = doInner(ctx)
	panicked = false
	return insertRes, err
}

func (m *Middleware) Work(ctx context.Context, job *rivertype.JobRow, doInner func(context.Context) error) error {
	spanName := prefix + "work"
	if m.config.EnableWorkSpanJobKindSuffix {
		spanName += "/" + job.Kind
	}

	ctx, span := m.tracer.Start(ctx, spanName,
		trace.WithSpanKind(trace.SpanKindConsumer))
	defer span.End()

	attrs := []attribute.KeyValue{
		attribute.Int("attempt", job.Attempt),
		attribute.String("kind", job.Kind),
		attribute.Int("priority", job.Priority),
		attribute.String("queue", job.Queue),
		attribute.String("status", ""), // replaced below
		attribute.StringSlice("tag", job.Tags),
	}
	const statusIndex = 4

	var (
		begin    = time.Now()
		err      error
		panicked = true // set to false if program leaves normally
	)
	defer func() {
		duration := m.durationInPreferredUnit(time.Since(begin))

		if err != nil {
			var (
				cancelErr *river.JobCancelError
				snoozeErr *river.JobSnoozeError
			)
			switch {
			case errors.As(err, &cancelErr):
				attrs = append(attrs, attribute.Bool("cancel", true))
			case errors.As(err, &snoozeErr):
				attrs = append(attrs, attribute.Bool("snooze", true))
			}
		}

		setStatus(attrs, statusIndex, span, panicked, err)

		{
			// Add some higher cardinality attributes to spans, but keep them
			// out of metrics given it's been traditional wisdom that metric
			// attribute sets shouldn't be too large.
			attrs := append(attrs,
				attribute.Int64("id", job.ID),
				attribute.String("created_at", job.CreatedAt.Format(time.RFC3339)),
				attribute.String("scheduled_at", job.ScheduledAt.Format(time.RFC3339)),
			)
			span.SetAttributes(attrs...) // set after finalizing status
		}

		// This allocates a new slice, so make sure to do it as few times as possible.
		measurementOpt := metric.WithAttributes(attrs...)

		m.metrics.workCount.Add(ctx, 1, measurementOpt)
		m.metrics.workDuration.Record(ctx, duration, measurementOpt)
		m.metrics.workDurationHistogram.Record(ctx, duration, measurementOpt)

		if m.config.EnableSemanticMetrics {
			measurementOpt := metric.WithAttributes(
				attribute.String("messaging.operation.name", "work"),
				attribute.String("messaging.system", "river"),
			)

			m.metrics.messagingClientConsumedMessages.Add(ctx, 1, measurementOpt)
			m.metrics.messagingClientOperationDuration.Record(ctx, duration, measurementOpt)
			m.metrics.messagingProcessDuration.Record(ctx, duration, measurementOpt)
		}
	}()

	err = doInner(ctx)
	panicked = false
	return err
}

func (m *Middleware) durationInPreferredUnit(duration time.Duration) float64 {
	switch m.config.DurationUnit {
	case "ms":
		return float64(duration.Milliseconds())
	case "s":
		fallthrough
	default:
		return duration.Seconds()
	}
}

func mustFloat64Gauge(meter metric.Meter, name string, options ...metric.Float64GaugeOption) metric.Float64Gauge {
	metric, err := meter.Float64Gauge(name, options...)
	if err != nil {
		panic(err)
	}
	return metric
}

func mustFloat64Histogram(meter metric.Meter, name string, options ...metric.Float64HistogramOption) metric.Float64Histogram {
	metric, err := meter.Float64Histogram(name, options...)
	if err != nil {
		panic(err)
	}
	return metric
}

func mustInt64Counter(meter metric.Meter, name string, options ...metric.Int64CounterOption) metric.Int64Counter {
	metric, err := meter.Int64Counter(name, options...)
	if err != nil {
		panic(err)
	}
	return metric
}

// Sets success status on the given span and within the set of attributes. The
// index of the status attribute is required ahead of time as a minor
// optimization.
func setStatus(attrs []attribute.KeyValue, statusIndex int, span trace.Span, panicked bool, err error) {
	if attrs[statusIndex].Key != "status" {
		panic("status attribute not at expected index; bug?") // protect against future regression
	}

	switch {
	case panicked:
		attrs[statusIndex] = attribute.String("status", "panic")
		span.SetStatus(codes.Error, "panic")
	case err != nil:
		attrs[statusIndex] = attribute.String("status", "error")
		span.SetStatus(codes.Error, err.Error())
	default:
		attrs[statusIndex] = attribute.String("status", "ok")
		span.SetStatus(codes.Ok, "")
	}
}
