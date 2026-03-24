package telemetry

import (
	"context"
	"log"
	"time"
)

// Config holds OpenTelemetry configuration.
type Config struct {
	Endpoint    string
	ServiceName string
}

// InitTracer initialises the OpenTelemetry tracer.
// Phase 1: logs configuration. Phase 2: connects to OTLP endpoint.
func InitTracer(cfg Config) func() {
	if cfg.Endpoint == "" {
		log.Println("OTEL: no endpoint configured, tracing disabled")
		return func() {}
	}

	log.Printf("OTEL: tracer configured for %s (service=%s)", cfg.Endpoint, cfg.ServiceName)
	// TODO Phase 2: Initialize OTLP gRPC exporter
	// tp := sdktrace.NewTracerProvider(...)
	// otel.SetTracerProvider(tp)

	return func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = ctx // Shutdown tracer provider
		log.Println("OTEL: tracer shutdown complete")
	}
}

// InitMeter initialises the OpenTelemetry meter for Prometheus metrics.
// Phase 1: logs configuration. Phase 2: serves /metrics endpoint.
func InitMeter(cfg Config) func() {
	if cfg.Endpoint == "" {
		log.Println("OTEL: no endpoint configured, metrics disabled")
		return func() {}
	}

	log.Printf("OTEL: meter configured for %s", cfg.Endpoint)
	// TODO Phase 2: Initialize Prometheus exporter on :9090/metrics
	// Mandatory metrics:
	//   ilgpt_query_duration_ms     histogram {provider, collection}
	//   ilgpt_llm_tokens_total      counter   {provider, direction}
	//   ilgpt_abstention_total      counter   {reason}
	//   ilgpt_ingest_docs_total     counter   {collection, status}
	//   ilgpt_qdrant_latency_ms     histogram {operation}
	//   ilgpt_ratelimit_hits_total  counter   {tier}

	return func() {
		log.Println("OTEL: meter shutdown complete")
	}
}
