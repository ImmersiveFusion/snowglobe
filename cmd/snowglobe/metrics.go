package main

import (
	"context"
	"crypto/tls"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/credentials"

	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
)

// ─── Configuration, set from flags in main ───────────────────────────────────

var (
	// metricsDisabled: when true, no OTel metrics are emitted at all.
	metricsDisabled bool

	// metricsInterval is the PeriodicReader flush interval. Metrics aggregate in
	// process and flush on this interval, so metric volume is decoupled from
	// -level: a level 10 run emits the same metric load as a level 1 run.
	metricsInterval time.Duration

	// metricsDelta selects delta aggregation temporality instead of the SDK
	// default (cumulative). Prometheus-backed stores are cumulative-native, while
	// some OTLP ingest paths ask producers for delta. Which one a given endpoint
	// actually implements is often undocumented, so this exists to settle it by
	// experiment rather than by reading.
	metricsDelta bool

	// metricsInstanceID adds service.instance.id (and host.name) to the metric
	// resource, giving per-pod resolution. Off by default and deliberately so:
	// it multiplies every series by the pod count, and metric backends commonly
	// bill per active series with no cardinality control on the ingest path.
	metricsInstanceID bool

	// metricsVerify emits the tracegen.spans.emitted oracle.
	metricsVerify bool
)

// ─── Instrument registry ─────────────────────────────────────────────────────

// serviceMetrics holds every instrument for one metric identity: a service by
// default, or a single pod when -metrics-instance-id is set.
type serviceMetrics struct {
	// Layer 0, derived from spans.
	requestDuration metric.Float64Histogram
	activeRequests  metric.Int64UpDownCounter
	spansEmitted    metric.Int64Counter

	// Layer 2, GenAI.
	tokenUsage    metric.Int64Histogram
	genAIDuration metric.Float64Histogram

	// Layer 1 state, observed by callbacks rather than recorded directly.
	// recentSpans drives simulated CPU: the gauge reads and resets it, so
	// utilization tracks what this service actually did in the last interval
	// instead of a random walk disconnected from the traces.
	recentSpans atomic.Int64
	inFlightDB  atomic.Int64
	memoryBytes atomic.Int64

	// queueDepth is per messaging destination. Producer spans increment it and
	// consumer spans decrement it, so with -no-consumers the backlog climbs
	// without bound while the trace graph still looks busy. That is the story
	// metrics tell here that traces cannot.
	queueMu    sync.Mutex
	queueDepth map[string]*atomic.Int64
}

var (
	metricsMu      sync.Mutex
	meterProviders []*sdkmetric.MeterProvider

	// metricsPool is keyed by metric identity: the service name by default, or
	// service/pod when -metrics-instance-id is set. The span processor carries
	// the same key, so the two cannot drift apart.
	metricsPool = map[string]*serviceMetrics{}

	// metricsByService indexes the same instrument sets by service alone, for
	// callers that know only the service. It mirrors tracerPool: pick a random
	// instance, the way tracer() does for multi-pod realism.
	metricsByService = map[string][]*serviceMetrics{}
)

// metricsForKey returns the instrument set for a metric identity, or nil when
// metrics are off or nothing was registered under that key.
func metricsForKey(key string) *serviceMetrics {
	if metricsDisabled {
		return nil
	}
	metricsMu.Lock()
	defer metricsMu.Unlock()
	return metricsPool[key]
}

// metricsFor returns a random instrument set for a service, mirroring tracer().
func metricsFor(svc string) *serviceMetrics {
	if metricsDisabled {
		return nil
	}
	metricsMu.Lock()
	defer metricsMu.Unlock()
	pool := metricsByService[svc]
	if len(pool) == 0 {
		return nil
	}
	return pool[rand.Intn(len(pool))]
}

// ─── Provider construction ───────────────────────────────────────────────────

// temporalitySelector maps every instrument kind to the configured temporality.
func temporalitySelector(sdkmetric.InstrumentKind) metricdata.Temporality {
	if metricsDelta {
		return metricdata.DeltaTemporality
	}
	return metricdata.CumulativeTemporality
}

// newMeterProvider builds the MeterProvider for one metric identity and
// registers every instrument against it.
//
// Providers are per SERVICE by default rather than per pod. The resource is what
// fixes series identity, so a per-pod provider would carry service.instance.id
// into every series whether or not anything wanted per-pod resolution. Opting in
// with -metrics-instance-id switches this to per pod.
func newMeterProvider(ctx context.Context, key, serviceName, instanceID, hostName, endpoint string) {
	opts := []otlpmetricgrpc.Option{
		otlpmetricgrpc.WithEndpoint(endpoint),
		otlpmetricgrpc.WithTemporalitySelector(temporalitySelector),
	}
	if len(otlpHeaders) > 0 {
		opts = append(opts, otlpmetricgrpc.WithHeaders(otlpHeaders))
	}
	if insecureMode {
		opts = append(opts, otlpmetricgrpc.WithInsecure())
	} else {
		opts = append(opts, otlpmetricgrpc.WithTLSCredentials(credentials.NewTLS(&tls.Config{})))
	}

	exporter, err := otlpmetricgrpc.New(ctx, opts...)
	if err != nil {
		fatal("failed to create metric exporter", "service", serviceName, "err", err)
	}

	attrs := []attribute.KeyValue{semconv.ServiceNameKey.String(serviceName)}
	if metricsInstanceID {
		attrs = append(attrs,
			attribute.String("service.instance.id", instanceID),
			attribute.String("host.name", hostName),
		)
	}

	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exporter,
			sdkmetric.WithInterval(metricsInterval))),
		sdkmetric.WithResource(resource.NewWithAttributes(semconv.SchemaURL, attrs...)),
	)
	meterProviders = append(meterProviders, mp)

	sm := newServiceMetrics(mp.Meter("tracegen"))

	metricsMu.Lock()
	metricsPool[key] = sm
	metricsByService[serviceName] = append(metricsByService[serviceName], sm)
	metricsMu.Unlock()
}

// newServiceMetrics creates every instrument against a meter. Split out from
// newMeterProvider so tests can drive it with a manual reader instead of an OTLP
// exporter, and assert on what the derivation actually produces.
// connectionPoolName is the value for the Required db.client.connection.pool.name
// dimension. Each simulated service has exactly one pool and its series are already
// scoped by the service.name resource attribute, so a constant costs no cardinality.
const connectionPoolName = "default"

func newServiceMetrics(meter metric.Meter) *serviceMetrics {
	var err error
	sm := &serviceMetrics{queueDepth: map[string]*atomic.Int64{}}
	sm.memoryBytes.Store(int64(180+rand.Intn(120)) << 20) // 180-300 MiB baseline

	// Layer 0. Explicit boundaries in SECONDS, from the HTTP semantic
	// conventions. Never inherit the SDK default layout: it is retained per
	// active series, which turns a cardinality problem into a memory problem.
	sm.requestDuration, err = meter.Float64Histogram(
		"http.server.request.duration",
		metric.WithUnit("s"),
		metric.WithDescription("Duration of HTTP server requests."),
		metric.WithExplicitBucketBoundaries(
			0.005, 0.01, 0.025, 0.05, 0.075, 0.1, 0.25, 0.5, 0.75, 1, 2.5, 5, 7.5, 10),
	)
	if err != nil {
		fatal("failed to create request duration histogram", "err", err)
	}

	sm.activeRequests, err = meter.Int64UpDownCounter(
		"http.server.active_requests",
		metric.WithUnit("{request}"),
		metric.WithDescription("Number of active HTTP server requests."),
	)
	if err != nil {
		fatal("failed to create active requests counter", "err", err)
	}

	if metricsVerify {
		sm.spansEmitted, err = meter.Int64Counter(
			"tracegen.spans.emitted",
			metric.WithUnit("{span}"),
			metric.WithDescription("Spans this generator finished. The emission oracle: diff it against what the pipeline counted."),
		)
		if err != nil {
			fatal("failed to create spans emitted counter", "err", err)
		}
	}

	// Layer 2. Bounded by construction: a handful of models across a handful of
	// systems and operations.
	sm.tokenUsage, err = meter.Int64Histogram(
		"gen_ai.client.token.usage",
		metric.WithUnit("{token}"),
		metric.WithDescription("Number of input and output tokens used."),
		metric.WithExplicitBucketBoundaries(
			1, 4, 16, 64, 256, 1024, 4096, 16384, 65536, 262144, 1048576, 4194304),
	)
	if err != nil {
		fatal("failed to create token usage histogram", "err", err)
	}

	sm.genAIDuration, err = meter.Float64Histogram(
		"gen_ai.client.operation.duration",
		metric.WithUnit("s"),
		metric.WithDescription("GenAI operation duration."),
		metric.WithExplicitBucketBoundaries(
			0.01, 0.02, 0.04, 0.08, 0.16, 0.32, 0.64, 1.28, 2.56, 5.12, 10.24, 20.48, 40.96, 81.92),
	)
	if err != nil {
		fatal("failed to create genai duration histogram", "err", err)
	}

	registerObservables(meter, sm)
	return sm
}

// registerObservables wires the layer 1 gauges. These describe state that spans
// structurally cannot carry, which is the reason this layer exists at all.
func registerObservables(meter metric.Meter, sm *serviceMetrics) {
	cpu, err := meter.Float64ObservableGauge(
		"system.cpu.utilization",
		metric.WithUnit("1"),
		metric.WithDescription("Simulated CPU utilization, driven by this service's recent span volume."),
	)
	if err != nil {
		fatal("failed to create cpu gauge", "err", err)
	}

	// UpDownCounter, not Gauge: the registry defines system.memory.usage as
	// instrument: updowncounter, which is a different point kind on the wire.
	// Reusing the reserved name with a gauge contract is the defect this fixes.
	mem, err := meter.Int64ObservableUpDownCounter(
		"system.memory.usage",
		metric.WithUnit("By"),
		metric.WithDescription("Simulated resident memory."),
	)
	if err != nil {
		fatal("failed to create memory usage counter", "err", err)
	}

	pool, err := meter.Int64ObservableUpDownCounter(
		"db.client.connection.count",
		metric.WithUnit("{connection}"),
		metric.WithDescription("Simulated database connection pool occupancy."),
	)
	if err != nil {
		fatal("failed to create db pool gauge", "err", err)
	}

	queue, err := meter.Int64ObservableGauge(
		"tracegen.messaging.queue.depth",
		metric.WithUnit("{message}"),
		metric.WithDescription("Messages published but not yet consumed. Climbs without bound under -no-consumers."),
	)
	if err != nil {
		fatal("failed to create queue depth gauge", "err", err)
	}

	_, err = meter.RegisterCallback(func(_ context.Context, o metric.Observer) error {
		// Read and reset: utilization reflects work done since the last flush.
		spans := sm.recentSpans.Swap(0)
		util := 0.04 + float64(spans)/400.0 + rand.Float64()*0.03
		if util > 0.98 {
			util = 0.98
		}
		o.ObserveFloat64(cpu, util, metric.WithAttributes(
			attribute.String("cpu.mode", "user")))

		// Memory drifts with load and leaks back down, staying inside a band so
		// the series looks alive without wandering off.
		delta := int64(spans)<<12 - (1 << 20) + int64(rand.Intn(1<<21))
		next := sm.memoryBytes.Add(delta)
		switch {
		case next < 120<<20:
			sm.memoryBytes.Store(120 << 20)
		case next > 900<<20:
			sm.memoryBytes.Store(900 << 20)
		}
		o.ObserveInt64(mem, sm.memoryBytes.Load(), metric.WithAttributes(
			attribute.String("system.memory.state", "used")))

		used := sm.inFlightDB.Load()
		if used > 20 {
			used = 20
		}
		o.ObserveInt64(pool, used, metric.WithAttributes(
			attribute.String("db.client.connection.state", "used"),
			attribute.String("db.client.connection.pool.name", connectionPoolName)))
		o.ObserveInt64(pool, 20-used, metric.WithAttributes(
			attribute.String("db.client.connection.state", "idle"),
			attribute.String("db.client.connection.pool.name", connectionPoolName)))

		sm.queueMu.Lock()
		for dest, depth := range sm.queueDepth {
			o.ObserveInt64(queue, depth.Load(), metric.WithAttributes(
				attribute.String("messaging.destination.name", dest)))
		}
		sm.queueMu.Unlock()

		return nil
	}, cpu, mem, pool, queue)
	if err != nil {
		fatal("failed to register metric callback", "err", err)
	}
}

// queueFor returns the depth counter for a destination, creating it on first use.
func (sm *serviceMetrics) queueFor(dest string) *atomic.Int64 {
	sm.queueMu.Lock()
	defer sm.queueMu.Unlock()
	d, ok := sm.queueDepth[dest]
	if !ok {
		d = &atomic.Int64{}
		sm.queueDepth[dest] = d
	}
	return d
}

// ─── Layer 0: metrics derived from spans ─────────────────────────────────────

// metricSpanProcessor derives metrics from finished spans. Deriving rather than
// hand-instrumenting means the ~200 scenario call sites stay untouched and the
// metrics agree with the traces by construction: the p99 in the histogram is the
// same span you can click into.
type metricSpanProcessor struct{ key string }

func (p metricSpanProcessor) OnStart(_ context.Context, s sdktrace.ReadWriteSpan) {
	sm := metricsForKey(p.key)
	if sm == nil {
		return
	}
	if s.SpanKind() == trace.SpanKindServer {
		sm.activeRequests.Add(context.Background(), 1, metric.WithAttributes(
			semconv.HTTPRequestMethodKey.String(spanMethod(s.Attributes())),
			semconv.URLSchemeKey.String("https")))
	}
}

func (p metricSpanProcessor) OnEnd(s sdktrace.ReadOnlySpan) {
	sm := metricsForKey(p.key)
	if sm == nil {
		return
	}

	sm.recentSpans.Add(1)
	attrs := s.Attributes()

	if metricsVerify && sm.spansEmitted != nil {
		sm.spansEmitted.Add(context.Background(), 1, metric.WithAttributes(
			attribute.String("span.kind", s.SpanKind().String())))
	}

	if s.SpanKind() == trace.SpanKindServer {
		sm.activeRequests.Add(context.Background(), -1, metric.WithAttributes(
			semconv.HTTPRequestMethodKey.String(spanMethod(attrs)),
			semconv.URLSchemeKey.String("https")))
		recordServerRequest(sm, s, attrs)
	}

	trackDatabase(sm, attrs)
	trackMessaging(sm, s, attrs)
	trackGenAI(sm, s, attrs)
}

func (metricSpanProcessor) Shutdown(context.Context) error   { return nil }
func (metricSpanProcessor) ForceFlush(context.Context) error { return nil }

// recordServerRequest emits http.server.request.duration for HTTP server spans.
//
// The attribute allowlist here is deliberate and must stay narrow. Spans carry
// user.id, order ids and session ids; copying a span's attribute set onto a
// metric is the classic way to mint one series per user.
func recordServerRequest(sm *serviceMetrics, s sdktrace.ReadOnlySpan, attrs []attribute.KeyValue) {
	route := attrValue(attrs, "http.route")
	if route == "" {
		return // not an HTTP server span, or no templated route: emitting the raw path would be a cardinality bomb
	}

	set := []attribute.KeyValue{
		semconv.HTTPRequestMethodKey.String(spanMethod(attrs)),
		semconv.URLSchemeKey.String("https"),
		semconv.HTTPRouteKey.String(route),
	}
	if code := attrInt(attrs, "http.response.status_code"); code != 0 {
		set = append(set, semconv.HTTPResponseStatusCodeKey.Int(code))
	}
	if s.Status().Code == codes.Error {
		set = append(set, semconv.ErrorTypeKey.String("_OTHER"))
	}

	sm.requestDuration.Record(context.Background(),
		s.EndTime().Sub(s.StartTime()).Seconds(),
		metric.WithAttributes(set...))
}

// trackDatabase keeps the simulated pool occupancy tied to real db span volume.
func trackDatabase(sm *serviceMetrics, attrs []attribute.KeyValue) {
	if attrValue(attrs, "db.system.name") == "" {
		return
	}
	sm.inFlightDB.Add(1)
	// Connections return to the pool shortly after the operation ends. Modeling
	// that with a timer keeps occupancy oscillating instead of ratcheting.
	time.AfterFunc(time.Duration(200+rand.Intn(800))*time.Millisecond, func() {
		sm.inFlightDB.Add(-1)
	})
}

// trackMessaging moves queue depth from the messaging spans themselves, which is
// what makes -no-consumers show up as an unbounded backlog: the publish spans
// keep arriving and the receive spans stop.
func trackMessaging(sm *serviceMetrics, s sdktrace.ReadOnlySpan, attrs []attribute.KeyValue) {
	dest := attrValue(attrs, "messaging.destination.name")
	if dest == "" {
		return
	}
	switch {
	case attrValue(attrs, "messaging.operation.type") == "send", s.SpanKind() == trace.SpanKindProducer:
		sm.queueFor(dest).Add(1)
	case attrValue(attrs, "messaging.operation.type") == "process", s.SpanKind() == trace.SpanKindConsumer:
		if d := sm.queueFor(dest); d.Load() > 0 {
			d.Add(-1)
		}
	}
}

// ─── Layer 2: GenAI ──────────────────────────────────────────────────────────

// trackGenAI derives the GenAI metrics from the span's own gen_ai attributes.
//
// The AI scenarios already stamp operation, system, model and token counts on
// their spans, so this needs no scenario changes and cannot disagree with the
// trace. Cardinality is naturally small: a handful of models across a handful of
// systems and operations.
func trackGenAI(sm *serviceMetrics, s sdktrace.ReadOnlySpan, attrs []attribute.KeyValue) {
	operation := attrValue(attrs, "gen_ai.operation.name")
	if operation == "" {
		return
	}

	base := []attribute.KeyValue{
		attribute.String("gen_ai.operation.name", operation),
		attribute.String("gen_ai.provider.name", attrValue(attrs, "gen_ai.provider.name")),
		attribute.String("gen_ai.request.model", attrValue(attrs, "gen_ai.request.model")),
	}

	ctx := context.Background()
	if in := attrInt(attrs, "gen_ai.usage.input_tokens"); in > 0 {
		sm.tokenUsage.Record(ctx, int64(in), metric.WithAttributes(
			append(append([]attribute.KeyValue{}, base...),
				attribute.String("gen_ai.token.type", "input"))...))
	}
	if out := attrInt(attrs, "gen_ai.usage.output_tokens"); out > 0 {
		sm.tokenUsage.Record(ctx, int64(out), metric.WithAttributes(
			append(append([]attribute.KeyValue{}, base...),
				attribute.String("gen_ai.token.type", "output"))...))
	}

	sm.genAIDuration.Record(ctx, s.EndTime().Sub(s.StartTime()).Seconds(),
		metric.WithAttributes(base...))
}

// ─── Attribute helpers ───────────────────────────────────────────────────────

// knownMethods bounds the http.request.method label. Semconv substitutes _OTHER
// for anything outside the set, so an unexpected verb cannot widen the series.
var knownMethods = map[string]bool{
	"GET": true, "HEAD": true, "POST": true, "PUT": true, "DELETE": true,
	"CONNECT": true, "OPTIONS": true, "TRACE": true, "PATCH": true,
}

func spanMethod(attrs []attribute.KeyValue) string {
	m := attrValue(attrs, "http.request.method")
	if m == "" {
		return "_OTHER"
	}
	if !knownMethods[m] {
		return "_OTHER"
	}
	return m
}

func attrValue(attrs []attribute.KeyValue, key string) string {
	for _, a := range attrs {
		if string(a.Key) == key {
			return a.Value.AsString()
		}
	}
	return ""
}

func attrInt(attrs []attribute.KeyValue, key string) int {
	for _, a := range attrs {
		if string(a.Key) == key {
			return int(a.Value.AsInt64())
		}
	}
	return 0
}

// shutdownMeterProviders flushes every provider on the way out. Metrics
// aggregate in memory between flushes, so without this the final interval is
// simply lost.
func shutdownMeterProviders() {
	for _, mp := range meterProviders {
		_ = mp.Shutdown(context.Background())
	}
}
