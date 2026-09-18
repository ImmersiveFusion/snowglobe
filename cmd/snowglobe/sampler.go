package main

import (
	"strings"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

// declaredFullSampler records and samples every span, exactly as the default
// sampler did, and says so on the wire: each span's tracestate carries the
// OpenTelemetry vendor entry ot=th:0 (sampling threshold 0, i.e. 100%).
// A backend reading the stream can then tell a complete trace stream from a
// probabilistically sampled one, so a span that never arrives reads as
// missing rather than as unsampled.
type declaredFullSampler struct{}

func (declaredFullSampler) ShouldSample(p sdktrace.SamplingParameters) sdktrace.SamplingResult {
	ts := trace.SpanContextFromContext(p.ParentContext).TraceState()
	// Insert places the member first, as W3C requires for a modified entry.
	// It only fails on an invalid key or value; if it ever does, keep the
	// parent's tracestate rather than dropping it.
	if updated, err := ts.Insert("ot", withTh0(ts.Get("ot"))); err == nil {
		ts = updated
	}
	return sdktrace.SamplingResult{
		Decision:   sdktrace.RecordAndSample,
		Tracestate: ts,
	}
}

func (declaredFullSampler) Description() string { return "DeclaredFull{ot=th:0}" }

// withTh0 returns the ot tracestate value with its th sub-key set to 0 and
// placed first. The value is ;-separated k:v sub-keys; any existing th is
// replaced and every other sub-key (such as rv) is kept in order.
func withTh0(v string) string {
	parts := []string{"th:0"}
	for _, kv := range strings.Split(v, ";") {
		if kv == "" || kv == "th" || strings.HasPrefix(kv, "th:") {
			continue
		}
		parts = append(parts, kv)
	}
	return strings.Join(parts, ";")
}
