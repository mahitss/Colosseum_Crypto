package telemetry

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"
)

var (
	// Metrics
	requestCount    metric.Int64Counter
	requestDuration metric.Float64Histogram
	pantaLatency    metric.Float64Histogram
	syncDuration    metric.Float64Histogram
	alertsTriggered metric.Int64Counter
	notifications   metric.Int64Counter
	errors          metric.Int64Counter

	meter = otel.Meter("prophet-gateway")
)

// InitTelemetry initializes OpenTelemetry tracing and metrics.
func InitTelemetry(ctx context.Context, serviceName string) (func(context.Context) error, error) {
	var shutdownFuncs []func(context.Context) error

	// Create resource
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion("1.0.0"),
			semconv.DeploymentEnvironment(os.Getenv("APP_ENV")),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("create resource: %w", err)
	}

	// Initialize tracing
	shutdownTracer, err := initTracer(ctx, res)
	if err != nil {
		return nil, err
	}
	shutdownFuncs = append(shutdownFuncs, shutdownTracer)

	// Initialize metrics
	shutdownMeter, err := initMetrics(ctx, res)
	if err != nil {
		return nil, err
	}
	shutdownFuncs = append(shutdownFuncs, shutdownMeter)

	// Initialize custom metrics
	initCustomMetrics()

	return func(ctx context.Context) error {
		var errs []error
		for _, fn := range shutdownFuncs {
			if err := fn(ctx); err != nil {
				errs = append(errs, err)
			}
		}
		if len(errs) > 0 {
			return fmt.Errorf("shutdown telemetry: %v", errs)
		}
		return nil
	}, nil
}

func initTracer(ctx context.Context, res *resource.Resource) (func(context.Context) error, error) {
	// Use OTLP HTTP exporter
	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if endpoint == "" {
		endpoint = "http://localhost:4318"
	}

	exporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpoint(endpoint),
		otlptracehttp.WithInsecure(),
	)
	if err != nil {
		return nil, fmt.Errorf("create trace exporter: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(0.1))),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return tp.Shutdown, nil
}

func initMetrics(ctx context.Context, res *resource.Resource) (func(context.Context) error, error) {
	exporter, err := prometheus.New()
	if err != nil {
		return nil, fmt.Errorf("create prometheus exporter: %w", err)
	}

	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(exporter),
		sdkmetric.WithResource(res),
	)

	otel.SetMeterProvider(mp)

	return mp.Shutdown, nil
}

func initCustomMetrics() {
	var err error

	requestCount, err = meter.Int64Counter(
		"http_requests_total",
		metric.WithDescription("Total number of HTTP requests"),
		metric.WithUnit("1"),
	)
	if err != nil {
		slog.Error("create request count metric", "error", err)
	}

	requestDuration, err = meter.Float64Histogram(
		"http_request_duration_seconds",
		metric.WithDescription("HTTP request duration in seconds"),
		metric.WithUnit("s"),
	)
	if err != nil {
		slog.Error("create request duration metric", "error", err)
	}

	pantaLatency, err = meter.Float64Histogram(
		"panta_request_duration_seconds",
		metric.WithDescription("Panta API request duration in seconds"),
		metric.WithUnit("s"),
	)
	if err != nil {
		slog.Error("create panta latency metric", "error", err)
	}

	syncDuration, err = meter.Float64Histogram(
		"worker_sync_duration_seconds",
		metric.WithDescription("Worker sync tick duration in seconds"),
		metric.WithUnit("s"),
	)
	if err != nil {
		slog.Error("create sync duration metric", "error", err)
	}

	alertsTriggered, err = meter.Int64Counter(
		"alerts_triggered_total",
		metric.WithDescription("Total number of alerts triggered"),
		metric.WithUnit("1"),
	)
	if err != nil {
		slog.Error("create alerts triggered metric", "error", err)
	}

	notifications, err = meter.Int64Counter(
		"notifications_delivered_total",
		metric.WithDescription("Total number of notifications delivered"),
		metric.WithUnit("1"),
	)
	if err != nil {
		slog.Error("create notifications metric", "error", err)
	}

	errors, err = meter.Int64Counter(
		"errors_total",
		metric.WithDescription("Total number of errors"),
		metric.WithUnit("1"),
	)
	if err != nil {
		slog.Error("create errors metric", "error", err)
	}
}

// RecordHTTPRequest records metrics for an HTTP request.
func RecordHTTPRequest(method, path string, statusCode int, duration time.Duration) {
	if requestCount != nil {
		requestCount.Add(context.Background(), 1,
			metric.WithAttributes(
				attribute.String("method", method),
				attribute.String("path", path),
				attribute.Int("status", statusCode),
			),
		)
	}
	if requestDuration != nil {
		requestDuration.Record(context.Background(), duration.Seconds(),
			metric.WithAttributes(
				attribute.String("method", method),
				attribute.String("path", path),
				attribute.Int("status", statusCode),
			),
		)
	}
}

// RecordPantaRequest records metrics for a Panta API request.
func RecordPantaRequest(operation string, duration time.Duration, err error) {
	if pantaLatency != nil {
		pantaLatency.Record(context.Background(), duration.Seconds(),
			metric.WithAttributes(
				attribute.String("operation", operation),
				attribute.Bool("error", err != nil),
			),
		)
	}
	if err != nil && errors != nil {
		errors.Add(context.Background(), 1,
			metric.WithAttributes(
				attribute.String("source", "panta"),
				attribute.String("operation", operation),
			),
		)
	}
}

// RecordSyncTick records metrics for a worker sync tick.
func RecordSyncTick(duration time.Duration, stats map[string]int) {
	if syncDuration != nil {
		syncDuration.Record(context.Background(), duration.Seconds())
	}
	if alertsTriggered != nil {
		if matched, ok := stats["alerts_matched"]; ok {
			alertsTriggered.Add(context.Background(), int64(matched))
		}
	}
	if notifications != nil {
		if delivered, ok := stats["notifications_delivered"]; ok {
			notifications.Add(context.Background(), int64(delivered))
		}
	}
}

// RecordError records an error metric.
func RecordError(source, operation string) {
	if errors != nil {
		errors.Add(context.Background(), 1,
			metric.WithAttributes(
				attribute.String("source", source),
				attribute.String("operation", operation),
			),
		)
	}
}

// TracingMiddleware returns middleware that adds trace context to requests.
func TracingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := otel.GetTextMapPropagator().Extract(r.Context(), propagation.HeaderCarrier(r.Header))
		tracer := otel.Tracer("prophet-gateway")

ctx, span := tracer.Start(ctx, "HTTP "+r.Method+" "+r.URL.Path,
		trace.WithAttributes(httpServerAttributes(r)...))
		defer span.End()

		ww := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(ww, r.WithContext(ctx))

		span.SetAttributes(
			attribute.Int("http.status_code", ww.statusCode),
		)
	})
}

func httpServerAttributes(r *http.Request) []attribute.KeyValue {
	attrs := []attribute.KeyValue{
		attribute.String("http.method", r.Method),
		attribute.String("http.scheme", "http"),
		attribute.String("http.target", r.URL.Path),
		attribute.String("http.flavor", r.Proto),
		attribute.String("net.host.name", r.Host),
	}
	if ua := r.UserAgent(); ua != "" {
		attrs = append(attrs, attribute.String("http.user_agent", ua))
	}
	return attrs
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	return rw.ResponseWriter.Write(b)
}