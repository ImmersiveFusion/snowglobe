package main

import (
	"context"
	"testing"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

func newDeclaredFullProvider() (*sdktrace.TracerProvider, *tracetest.SpanRecorder) {
	rec := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(declaredFullSampler{}),
		sdktrace.WithSpanProcessor(rec),
	)
	return tp, rec
}

func TestDeclaredFullRootSpanCarriesTh0(t *testing.T) {
	tp, _ := newDeclaredFullProvider()
	defer tp.Shutdown(context.Background())

	_, span := tp.Tracer("t").Start(context.Background(), "root")
	span.End()

	if got := span.SpanContext().TraceState().Get("ot"); got != "th:0" {
		t.Fatalf("root ot = %q, want th:0", got)
	}
}

func TestDeclaredFullChildSpanCarriesTh0(t *testing.T) {
	tp, _ := newDeclaredFullProvider()
	defer tp.Shutdown(context.Background())
	// Snowglobe runs one provider per service instance, so a downstream hop
	// starts its span from a different provider with the caller's context.
	other, _ := newDeclaredFullProvider()
	defer other.Shutdown(context.Background())

	ctx, root := tp.Tracer("t").Start(context.Background(), "root")
	_, child := tp.Tracer("t").Start(ctx, "child")
	_, remote := other.Tracer("other").Start(ctx, "downstream")
	child.End()
	remote.End()
	root.End()

	for name, s := range map[string]trace.Span{"same-provider child": child, "other-provider child": remote} {
		if got := s.SpanContext().TraceState().Get("ot"); got != "th:0" {
			t.Fatalf("%s ot = %q, want th:0", name, got)
		}
		if s.SpanContext().TraceID() != root.SpanContext().TraceID() {
			t.Fatalf("%s left the parent's trace", name)
		}
	}
}

func TestDeclaredFullKeepsParentTracestateWithOtFirst(t *testing.T) {
	tp, _ := newDeclaredFullProvider()
	defer tp.Shutdown(context.Background())

	ts, err := trace.ParseTraceState("foo=bar")
	if err != nil {
		t.Fatal(err)
	}
	parent := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    trace.TraceID{1},
		SpanID:     trace.SpanID{1},
		TraceFlags: trace.FlagsSampled,
		TraceState: ts,
		Remote:     true,
	})
	ctx := trace.ContextWithRemoteSpanContext(context.Background(), parent)
	_, span := tp.Tracer("t").Start(ctx, "child")
	span.End()

	if got := span.SpanContext().TraceState().String(); got != "ot=th:0,foo=bar" {
		t.Fatalf("tracestate = %q, want ot=th:0,foo=bar", got)
	}
}

func TestWithTh0(t *testing.T) {
	cases := []struct{ in, want string }{
		{"rv:abc123", "th:0;rv:abc123"},
		{"th:8", "th:0"},
		{"", "th:0"},
		{"rv:abc123;th:8", "th:0;rv:abc123"},
	}
	for _, c := range cases {
		if got := withTh0(c.in); got != c.want {
			t.Errorf("withTh0(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestDeclaredFullRecordsAndSamplesEverySpan(t *testing.T) {
	tp, rec := newDeclaredFullProvider()
	defer tp.Shutdown(context.Background())

	const n = 25
	tr := tp.Tracer("t")
	ctx, root := tr.Start(context.Background(), "root")
	for i := 0; i < n-1; i++ {
		_, s := tr.Start(ctx, "child")
		if !s.IsRecording() || !s.SpanContext().IsSampled() {
			t.Fatalf("span %d not recorded and sampled", i)
		}
		s.End()
	}
	root.End()

	if got := len(rec.Ended()); got != n {
		t.Fatalf("recorded %d spans, want %d", got, n)
	}
	for _, s := range rec.Ended() {
		if !s.SpanContext().IsSampled() {
			t.Fatalf("span %q not sampled", s.Name())
		}
	}
}
