package main

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

func TestSpanMethodBoundsTheLabel(t *testing.T) {
	// http.request.method must stay bounded: an unbounded verb would mint one
	// series per value a client cares to invent.
	cases := []struct {
		name  string
		attrs []attribute.KeyValue
		want  string
	}{
		{"known verb", []attribute.KeyValue{attribute.String("http.request.method", "GET")}, "GET"},
		{"another known verb", []attribute.KeyValue{attribute.String("http.request.method", "PATCH")}, "PATCH"},
		{"unknown verb", []attribute.KeyValue{attribute.String("http.request.method", "FROBNICATE")}, "_OTHER"},
		{"injection attempt", []attribute.KeyValue{attribute.String("http.request.method", "' OR 1=1")}, "_OTHER"},
		{"absent", nil, "_OTHER"},
		{"empty", []attribute.KeyValue{attribute.String("http.request.method", "")}, "_OTHER"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := spanMethod(c.attrs); got != c.want {
				t.Fatalf("spanMethod(%v) = %q, want %q", c.attrs, got, c.want)
			}
		})
	}
}

func TestSpanMethodIsCaseSensitiveToTheAllowlist(t *testing.T) {
	// Tracegen emits uppercase verbs. A lowercase one is not silently accepted:
	// letting case through would double the series for every route.
	attrs := []attribute.KeyValue{attribute.String("http.request.method", "get")}
	if got := spanMethod(attrs); got != "_OTHER" {
		t.Fatalf("spanMethod(get) = %q, want _OTHER", got)
	}
}

func TestAttrValue(t *testing.T) {
	attrs := []attribute.KeyValue{
		attribute.String("http.route", "/users/{id}"),
		attribute.String("db.system.name", "postgresql"),
		attribute.Int("http.response.status_code", 200),
	}
	if got := attrValue(attrs, "http.route"); got != "/users/{id}" {
		t.Fatalf("attrValue(http.route) = %q", got)
	}
	if got := attrValue(attrs, "missing"); got != "" {
		t.Fatalf("attrValue(missing) = %q, want empty", got)
	}
}

func TestAttrInt(t *testing.T) {
	attrs := []attribute.KeyValue{attribute.Int("http.response.status_code", 503)}
	if got := attrInt(attrs, "http.response.status_code"); got != 503 {
		t.Fatalf("attrInt = %d, want 503", got)
	}
	// Absent returns the zero value, which recordServerRequest treats as "no
	// status code" and omits from the attribute set rather than reporting a 0.
	if got := attrInt(attrs, "missing"); got != 0 {
		t.Fatalf("attrInt(missing) = %d, want 0", got)
	}
}

func TestTemporalitySelectorFollowsTheFlag(t *testing.T) {
	// Prometheus-backed stores are cumulative-native; some OTLP paths ask for
	// delta. Getting this backwards yields zeros and phantom reset spikes rather
	// than an error, so it is worth pinning.
	orig := metricsDelta
	t.Cleanup(func() { metricsDelta = orig })

	metricsDelta = false
	if got := temporalitySelector(sdkmetric.InstrumentKindCounter); got != metricdata.CumulativeTemporality {
		t.Fatalf("default temporality = %v, want cumulative", got)
	}

	metricsDelta = true
	if got := temporalitySelector(sdkmetric.InstrumentKindCounter); got != metricdata.DeltaTemporality {
		t.Fatalf("delta temporality = %v, want delta", got)
	}
}

func TestQueueForIsStableAndConcurrencySafe(t *testing.T) {
	// Queue depth is incremented and decremented from the span processor, which
	// runs on every scenario goroutine at once.
	sm := &serviceMetrics{queueDepth: map[string]*atomic.Int64{}}

	first := sm.queueFor("orders.created")
	second := sm.queueFor("orders.created")
	if first != second {
		t.Fatal("queueFor returned a different counter for the same destination")
	}

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sm.queueFor("orders.created").Add(1)
			sm.queueFor("inventory.reserve").Add(1)
		}()
	}
	wg.Wait()

	if got := sm.queueFor("orders.created").Load(); got != 50 {
		t.Fatalf("orders.created depth = %d, want 50", got)
	}
	if got := sm.queueFor("inventory.reserve").Load(); got != 50 {
		t.Fatalf("inventory.reserve depth = %d, want 50", got)
	}
}

func TestMetricsLookupsAreNilWhenDisabled(t *testing.T) {
	// Every recording path checks for nil, so disabling metrics must produce nil
	// rather than an empty instrument set that would panic on use.
	orig := metricsDisabled
	t.Cleanup(func() { metricsDisabled = orig })

	metricsDisabled = true
	if sm := metricsFor("web-frontend"); sm != nil {
		t.Fatal("metricsFor returned non-nil with metrics disabled")
	}
	if sm := metricsForKey("web-frontend"); sm != nil {
		t.Fatal("metricsForKey returned non-nil with metrics disabled")
	}
}

func TestMetricsForUnknownServiceIsNil(t *testing.T) {
	// Services outside the active complexity tier are never registered. The
	// recording paths must tolerate that, the way tracer() returns a noop.
	orig := metricsDisabled
	t.Cleanup(func() { metricsDisabled = orig })
	metricsDisabled = false

	if sm := metricsFor("service-that-does-not-exist"); sm != nil {
		t.Fatal("metricsFor returned non-nil for an unregistered service")
	}
}

// registerTestMetrics wires a serviceMetrics backed by a manual reader, so a
// test can drive real spans through the processor and read back what the
// derivation actually produced.
func registerTestMetrics(t *testing.T, key string) (*sdkmetric.ManualReader, *serviceMetrics) {
	t.Helper()
	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	sm := newServiceMetrics(mp.Meter("test"))

	metricsMu.Lock()
	metricsPool[key] = sm
	metricsMu.Unlock()
	t.Cleanup(func() {
		metricsMu.Lock()
		delete(metricsPool, key)
		metricsMu.Unlock()
	})
	return reader, sm
}

// findMetric returns the named metric from a collected resource, or nil.
func findMetric(rm *metricdata.ResourceMetrics, name string) *metricdata.Metrics {
	for i := range rm.ScopeMetrics {
		for j := range rm.ScopeMetrics[i].Metrics {
			if rm.ScopeMetrics[i].Metrics[j].Name == name {
				return &rm.ScopeMetrics[i].Metrics[j]
			}
		}
	}
	return nil
}

func TestServerSpanDerivesRequestDuration(t *testing.T) {
	// The whole point of deriving rather than hand-instrumenting: a scenario span
	// produces the metric with no scenario code involved.
	orig := metricsDisabled
	t.Cleanup(func() { metricsDisabled = orig })
	metricsDisabled = false

	reader, _ := registerTestMetrics(t, "svc-under-test")

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSpanProcessor(metricSpanProcessor{key: "svc-under-test"}))
	_, span := tp.Tracer("t").Start(context.Background(), "POST /api/v2/orders",
		trace.WithSpanKind(trace.SpanKindServer),
		trace.WithAttributes(
			attribute.String("http.request.method", "POST"),
			attribute.String("http.route", "/api/v2/orders"),
			attribute.Int("http.response.status_code", 200),
			attribute.String("user.id", "usr_000123"), // must NOT reach the metric
		))
	span.End()

	var rm metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &rm); err != nil {
		t.Fatalf("collect: %v", err)
	}

	m := findMetric(&rm, "http.server.request.duration")
	if m == nil {
		t.Fatal("http.server.request.duration was not recorded")
	}
	if m.Unit != "s" {
		t.Fatalf("unit = %q, want s (a milliseconds histogram under this name is misread by 1000x)", m.Unit)
	}

	hist, ok := m.Data.(metricdata.Histogram[float64])
	if !ok {
		t.Fatalf("data type = %T, want Histogram[float64]", m.Data)
	}
	if len(hist.DataPoints) != 1 {
		t.Fatalf("data points = %d, want 1", len(hist.DataPoints))
	}

	dp := hist.DataPoints[0]
	if dp.Count != 1 {
		t.Fatalf("count = %d, want 1", dp.Count)
	}

	got := map[string]string{}
	for _, kv := range dp.Attributes.ToSlice() {
		got[string(kv.Key)] = kv.Value.Emit()
	}
	for k, want := range map[string]string{
		"http.request.method":       "POST",
		"http.route":                "/api/v2/orders",
		"http.response.status_code": "200",
	} {
		if got[k] != want {
			t.Errorf("attribute %s = %q, want %q", k, got[k], want)
		}
	}
	// The cardinality guard: span attributes are NOT copied onto the metric.
	if _, leaked := got["user.id"]; leaked {
		t.Error("user.id leaked onto the metric; that is one series per user")
	}
}

func TestSpanWithoutTemplatedRouteIsNotRecorded(t *testing.T) {
	// No templated route means no safe series to emit. Recording the raw path
	// instead would mint one series per URL.
	orig := metricsDisabled
	t.Cleanup(func() { metricsDisabled = orig })
	metricsDisabled = false

	reader, _ := registerTestMetrics(t, "svc-no-route")

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSpanProcessor(metricSpanProcessor{key: "svc-no-route"}))
	_, span := tp.Tracer("t").Start(context.Background(), "GET /orders/12345",
		trace.WithSpanKind(trace.SpanKindServer),
		trace.WithAttributes(attribute.String("http.request.method", "GET")))
	span.End()

	var rm metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &rm); err != nil {
		t.Fatalf("collect: %v", err)
	}
	if m := findMetric(&rm, "http.server.request.duration"); m != nil {
		t.Fatal("recorded a duration for a span with no templated route")
	}
}

func TestMessagingSpansMoveQueueDepth(t *testing.T) {
	// This is what makes -no-consumers legible: publish increments, receive
	// decrements, so a dead consumer shows up as an unbounded backlog.
	orig := metricsDisabled
	t.Cleanup(func() { metricsDisabled = orig })
	metricsDisabled = false

	_, sm := registerTestMetrics(t, "svc-messaging")
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSpanProcessor(metricSpanProcessor{key: "svc-messaging"}))

	// name is the free-text system verb; type is the registered enum the
	// derivation classifies on (send/process, not publish/receive).
	emit := func(op string) {
		opType := map[string]string{"publish": "send", "receive": "process"}[op]
		_, s := tp.Tracer("t").Start(context.Background(), "msg",
			trace.WithAttributes(
				attribute.String("messaging.destination.name", "orders.created"),
				attribute.String("messaging.operation.name", op),
				attribute.String("messaging.operation.type", opType)))
		s.End()
	}

	for i := 0; i < 5; i++ {
		emit("publish")
	}
	if got := sm.queueFor("orders.created").Load(); got != 5 {
		t.Fatalf("after 5 publishes depth = %d, want 5", got)
	}

	emit("receive")
	emit("receive")
	if got := sm.queueFor("orders.created").Load(); got != 3 {
		t.Fatalf("after 2 receives depth = %d, want 3", got)
	}
}

func TestGenAISpanDerivesTokenUsage(t *testing.T) {
	// The AI scenarios already stamp these on the span, so the metric needs no
	// scenario changes and cannot disagree with the trace.
	orig := metricsDisabled
	t.Cleanup(func() { metricsDisabled = orig })
	metricsDisabled = false

	reader, _ := registerTestMetrics(t, "llm-gateway-test")
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSpanProcessor(metricSpanProcessor{key: "llm-gateway-test"}))

	_, span := tp.Tracer("t").Start(context.Background(), "chat gpt-5.4",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("gen_ai.operation.name", "chat"),
			attribute.String("gen_ai.provider.name", "openai"),
			attribute.String("gen_ai.request.model", "gpt-5.4"),
			attribute.Int("gen_ai.usage.input_tokens", 1200),
			attribute.Int("gen_ai.usage.output_tokens", 340),
		))
	span.End()

	var rm metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &rm); err != nil {
		t.Fatalf("collect: %v", err)
	}

	m := findMetric(&rm, "gen_ai.client.token.usage")
	if m == nil {
		t.Fatal("gen_ai.client.token.usage was not recorded")
	}
	hist, ok := m.Data.(metricdata.Histogram[int64])
	if !ok {
		t.Fatalf("data type = %T, want Histogram[int64]", m.Data)
	}
	// One series for input tokens, one for output.
	if len(hist.DataPoints) != 2 {
		t.Fatalf("data points = %d, want 2 (input and output)", len(hist.DataPoints))
	}

	sums := map[string]int64{}
	for _, dp := range hist.DataPoints {
		for _, kv := range dp.Attributes.ToSlice() {
			if kv.Key == "gen_ai.token.type" {
				sums[kv.Value.Emit()] = dp.Sum
			}
		}
	}
	if sums["input"] != 1200 {
		t.Errorf("input tokens = %d, want 1200", sums["input"])
	}
	if sums["output"] != 340 {
		t.Errorf("output tokens = %d, want 340", sums["output"])
	}
}
