// http://www.inanzzz.com/index.php/post/4qes/implementing-opentelemetry-and-jaeger-tracing-in-golang-http-api
package trace

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.12.0"
	"go.opentelemetry.io/otel/trace"

	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

var tracer trace.Tracer

// ProviderConfig represents the provider configuration and used to create a new
// `Provider` type.
type ProviderConfig struct {
	JaegerEndpoint string
	ServiceName    string
	ServiceVersion string
	Environment    string
	// Set this to `true` if you want to disable tracing completly.
	Disabled bool
}

// Provider represents the tracer provider. Depending on the `config.Disabled`
// parameter, it will either use a "live" provider or a "no operations" version.
// The "no operations" means, tracing will be globally disabled.
type Provider struct {
	provider trace.TracerProvider
}

// New returns a new `Provider` type. It uses Jaeger exporter and globally sets
// the tracer provider as well as the global tracer for spans.
func NewProvider(config ProviderConfig) (Provider, error) {
	if config.Disabled {
		return Provider{provider: trace.NewNoopTracerProvider()}, nil
	}

	exp, err := jaeger.New(
		jaeger.WithCollectorEndpoint(jaeger.WithEndpoint(config.JaegerEndpoint)),
	)
	if err != nil {
		return Provider{}, err
	}

	prv := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(sdkresource.NewWithAttributes(
			"",
			semconv.ServiceNameKey.String(config.ServiceName),
			semconv.ServiceVersionKey.String(config.ServiceVersion),
			semconv.DeploymentEnvironmentKey.String(config.Environment),
		)),
	)

	otel.SetTracerProvider(prv)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	tracer = struct{ trace.Tracer }{otel.Tracer(config.ServiceName)}

	return Provider{provider: prv}, nil
}

// Close shuts down the tracer provider only if it was not "no operations"
// version.
func (p Provider) Close(ctx context.Context) error {
	if prv, ok := p.provider.(*sdktrace.TracerProvider); ok {
		return prv.Shutdown(ctx)
	}

	return nil
}

func MakeTraceContextToCarrier(ctx context.Context) map[string]string {
	textCtxCarrier := propagation.MapCarrier(make(map[string]string))
	traceContext := propagation.TraceContext{}
	traceContext.Inject(ctx, textCtxCarrier)
	return textCtxCarrier
}

func ExtractTraceContextFromCarrier(carrier map[string]string) context.Context {
	textCtxCarrier := propagation.MapCarrier(carrier)
	traceContext := propagation.TraceContext{}
	return traceContext.Extract(context.Background(), textCtxCarrier)
}

// Interfaces for span

type SpanLink trace.Link

func NewSpanLink(ctx context.Context) SpanLink {
	return SpanLink(trace.LinkFromContext(ctx))
}

func (sl SpanLink) ContextWithRemoteSpan(parent context.Context) context.Context {
	return trace.ContextWithRemoteSpanContext(parent, trace.Link(sl).SpanContext)
}

// SpanCustomiser is used to enforce custom span options. Any custom concrete
// span customiser type must implement this interface.
type SpanCustomiser interface {
	customise() trace.SpanStartOption
}

type SpanOptsBuilder interface {
	SpanCustomiser
	WithLink(link SpanLink) SpanOptsBuilder
}

type spanOptsBuilderImpl struct {
	links []trace.Link
}

func NewSpanOptsBuilder() SpanOptsBuilder {
	return &spanOptsBuilderImpl{}
}

func (sob *spanOptsBuilderImpl) customise() trace.SpanStartOption {
	return trace.WithLinks(sob.links...)
}

func (sob *spanOptsBuilderImpl) WithLink(link SpanLink) SpanOptsBuilder {
	sob.links = append(sob.links, trace.Link(link))
	return sob
}

// NewSpan returns a new span from the global tracer. Depending on the `cus`
// argument, the span is either a plain one or a customised one. Each resulting
// span must be completed with `defer span.End()` right after the call.
func NewSpan(ctx context.Context, name string, cus ...SpanCustomiser) (context.Context, trace.Span) {
	if len(cus) == 0 {
		return tracer.Start(ctx, name)
	}
	opts := []trace.SpanStartOption{}
	for _, c := range cus {
		opts = append(opts, c.customise())
	}
	return tracer.Start(ctx, name, opts...)
}

// func Init() {
// 	ctx := context.Background()

// 	// Bootstrap tracer.
// 	prv, err := NewProvider(ProviderConfig{
// 		JaegerEndpoint: "http://localhost:14268/api/traces",
// 		ServiceName:    "client",
// 		ServiceVersion: "1.0.0",
// 		Environment:    "dev",
// 		Disabled:       false,
// 	})
// 	if err != nil {
// 		log.Fatalln(err)
// 	}
// 	defer prv.Close(ctx)
// }
