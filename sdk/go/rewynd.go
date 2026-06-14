// Package rewynd wires a Go service's OpenTelemetry traces to a local rewynd core in one call.
//
// Go has no runtime auto-instrumentation, so unlike the Node and Python shims this is minimal
// setup, not zero setup: call Start at boot, then wrap your HTTP handler with otelhttp and your
// database with otelsql (or any OpenTelemetry instrumentation). Start owns the painful part —
// the exporter, resource, provider, batching, and flush-on-exit.
//
//	shutdown, _ := rewynd.Start(ctx)
//	defer shutdown(context.Background())
//	http.ListenAndServe(":8080", otelhttp.NewHandler(mux, "server"))
package rewynd

import (
	"context"
	"os"
	"path/filepath"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// Options configure Start; the zero value is the right default for local development.
type Options struct {
	// Service is the service.name reported to rewynd. Defaults to OTEL_SERVICE_NAME, then the
	// executable name.
	Service string
	// Endpoint is the OTLP/gRPC address of the rewynd core. Defaults to OTEL_EXPORTER_OTLP_ENDPOINT,
	// then 127.0.0.1:4317.
	Endpoint string
}

// Option mutates Options.
type Option func(*Options)

// WithService sets the reported service.name.
func WithService(name string) Option { return func(o *Options) { o.Service = name } }

// WithEndpoint points at a non-default rewynd core.
func WithEndpoint(addr string) Option { return func(o *Options) { o.Endpoint = addr } }

// Start registers a global tracer provider that exports to a local rewynd core and returns a
// shutdown that flushes — call it on exit so short-lived programs don't lose spans. It is a
// no-op (and returns a no-op shutdown) when REWYND_DISABLED is set, so it is safe to leave in.
func Start(ctx context.Context, opts ...Option) (func(context.Context) error, error) {
	noop := func(context.Context) error { return nil }
	if os.Getenv("REWYND_DISABLED") != "" {
		return noop, nil
	}

	o := Options{Service: defaultService(), Endpoint: defaultEndpoint()}
	for _, fn := range opts {
		fn(&o)
	}

	exp, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(o.Endpoint),
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		return noop, err
	}

	res, err := resource.New(ctx, resource.WithAttributes(semconv.ServiceName(o.Service)))
	if err != nil {
		res = resource.NewSchemaless(semconv.ServiceName(o.Service))
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)

	return tp.Shutdown, nil
}

func defaultService() string {
	if s := os.Getenv("OTEL_SERVICE_NAME"); s != "" {
		return s
	}
	if s := os.Getenv("REWYND_SERVICE"); s != "" {
		return s
	}
	if len(os.Args) > 0 {
		return filepath.Base(os.Args[0])
	}
	return "go-service"
}

func defaultEndpoint() string {
	if e := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"); e != "" {
		return e
	}
	return "127.0.0.1:4317"
}
