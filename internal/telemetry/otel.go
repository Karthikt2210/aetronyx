package telemetry

import (
	"context"
	"log/slog"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// Provider wraps the OpenTelemetry TracerProvider.
type Provider struct {
	tp *sdktrace.TracerProvider
}

// Init initializes the OTel provider. If enabled is false, it installs a no-op tracer.
// If enabled is true, it creates a stdout exporter and returns a configured Provider.
func Init(serviceName, version string, enabled bool) (*Provider, error) {
	if !enabled {
		// Install no-op tracer provider
		noopTP := sdktrace.NewTracerProvider()
		otel.SetTracerProvider(noopTP)
		return &Provider{tp: noopTP}, nil
	}

	// Create resource with service attributes
	res, err := resource.New(context.Background(),
		resource.WithAttributes(
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion(version),
		),
	)
	if err != nil {
		slog.Error("failed to create otel resource", "error", err)
		return nil, err
	}

	// Create stdout exporter
	exporter, err := stdouttrace.New()
	if err != nil {
		slog.Error("failed to create stdout exporter", "error", err)
		return nil, err
	}

	// Create tracer provider with stdout exporter
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithBatcher(exporter),
	)

	// Set as global tracer provider
	otel.SetTracerProvider(tp)

	return &Provider{tp: tp}, nil
}

// Tracer returns a tracer with the given name.
func (p *Provider) Tracer(name string) trace.Tracer {
	return p.tp.Tracer(name)
}

// Shutdown gracefully shuts down the provider and flushes any pending spans.
func (p *Provider) Shutdown(ctx context.Context) error {
	if p.tp == nil {
		return nil
	}
	return p.tp.Shutdown(ctx)
}

// SpanFromEvent creates a new span from an event with the given type and attributes.
// It returns the updated context and the span.
func (p *Provider) SpanFromEvent(ctx context.Context, eventType string, attrs map[string]any) (context.Context, trace.Span) {
	tracer := p.Tracer("event")

	// Convert map[string]any to []attribute.KeyValue
	var kvs []attribute.KeyValue
	kvs = append(kvs, attribute.String("event.type", eventType))
	for k, v := range attrs {
		switch val := v.(type) {
		case string:
			kvs = append(kvs, attribute.String(k, val))
		case int:
			kvs = append(kvs, attribute.Int(k, val))
		case float64:
			kvs = append(kvs, attribute.Float64(k, val))
		case bool:
			kvs = append(kvs, attribute.Bool(k, val))
		default:
			// Skip types we can't convert
		}
	}

	newCtx, span := tracer.Start(ctx, eventType, trace.WithAttributes(kvs...))
	return newCtx, span
}
