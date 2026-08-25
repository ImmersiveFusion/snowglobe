# Metrics: design

Status: **implemented**. See `cmd/snowglobe/metrics.go`.

This document covers what Snowglobe emits as OTLP metrics, why, and the constraints that shape it.

## What changed between design and implementation

Two things landed differently from the plan below, both improvements worth recording.

**Everything is derived from spans, including the infrastructure layer.** The plan described layer 1 as
a background sampler producing plausible-looking numbers. It is instead driven by real scenario
activity: CPU utilization reads a counter of spans the service actually finished in the last interval,
database pool occupancy tracks in-flight database spans, and queue depth is incremented by producer
spans and decremented by consumer spans. That last one means `-no-consumers` produces a genuinely
unbounded backlog rather than a simulated one, and it costs less code than a random walk would have.

**GenAI metrics need no scenario changes either.** The AI scenarios already stamp
`gen_ai.operation.name`, `gen_ai.provider.name`, `gen_ai.request.model` and token counts on their spans, so
the same span processor derives the metrics. The net result is that all four layers are implemented
with **zero edits to the ~3,000 lines of scenario code**.

One item from the plan was dropped: cache hit ratio. Snowglobe's cache spans do not distinguish a hit
from a miss, so a counter pair would have been invented rather than derived. It needs a span attribute
first.

---

## What exists today

`newProvider()` in [`cmd/snowglobe/main.go`](../cmd/snowglobe/main.go) builds a `TracerProvider` and a
`LoggerProvider` per **pod**, sharing one resource (`service.name`, `service.instance.id`, `host.name`).
Depending on `-complexity` that is roughly 20 to 59 pods across 10 to 28 services. Scenarios call
`tracer(svc).Start(...)` directly, at around 200 call sites.

Both signals export per event: traces via `WithSyncer` (a simple span processor), logs via
`NewSimpleProcessor`.

Metrics do not work that way, and that difference is the single most important design fact here.

---

## Why metrics are not just a third exporter

Traces and logs are push-per-event, so their volume scales with `-level`. Metrics aggregate in process
and flush on an interval via a `PeriodicReader`. A `-level 10` run produces roughly 350 traces/second
but the **same** metric volume as `-level 1`.

That is a feature. It means the metrics load is predictable and decoupled from the trace load, so a
metrics-enabled Snowglobe does not multiply the cost of a high-rate demo grid. It also means metric
volume is governed almost entirely by **series count**, which is why the cardinality budget below is
the binding constraint rather than the tick rate.

---

## Decisions already taken

| Decision | Value |
|---|---|
| Scope | RED + infrastructure + GenAI |
| Primary driver | Ingest into a Prometheus-backed OTLP endpoint |
| Toggle | `-no-metrics` (on by default, mirroring `-no-logs`) |
| Interval | `-metrics-interval`, default 15s |

---

## Naming: conform where a convention exists, private namespace otherwise

Two rules, and the split matters more than either rule alone.

**Where OpenTelemetry defines a metric, use its name and conform to its contract exactly.** That means
`http.server.request.duration` as a Histogram in **seconds** with the spec's recommended boundaries
`[0.005, 0.01, 0.025, 0.05, 0.075, 0.1, 0.25, 0.5, 0.75, 1, 2.5, 5, 7.5, 10]`, carrying
`http.request.method` and `url.scheme`, plus `http.response.status_code`, `http.route`, and `error.type`
where applicable. Emitting that name in milliseconds would make every OTel-aware dashboard misread it by
1000x. Conformance is not stylistic here, it is the whole point of using the reserved name.

**Everything else goes under a `tracegen.` prefix.** The spec's guidance for names outside the
conventions is to prefix with a reverse domain or an application name unique within the organisation,
and it explicitly warns against extending an existing OTel namespace with a non-standard metric. So
queue depth becomes `tracegen.messaging.queue.depth`, not `messaging.queue.depth`.

Instrument names use dots. Do not pre-normalise to underscores: some backends preserve dots to the query
surface and some translate them, and that is the backend's job, not the producer's.

---

## Layer 1: infrastructure metrics

The genuinely new signal, and the reason to build this at all. These describe state that spans
structurally cannot carry, sampled by a per-pod background goroutine rather than derived from scenario
execution.

| Metric | Instrument | Unit | Notes |
|---|---|---|---|
| `system.cpu.utilization` | Gauge | `1` | Per pod, correlated with that pod's scenario activity |
| `system.memory.usage` | UpDownCounter | `By` | `system.memory.state`. The registry defines this as an updowncounter, not a gauge |
| `http.server.active_requests` | UpDownCounter | `{request}` | Semconv; rises and falls with in-flight scenarios |
| `db.client.connection.count` | UpDownCounter | `{connection}` | Semconv, with `db.client.connection.state` = `used`/`idle` |
| `tracegen.messaging.queue.depth` | Gauge | `{message}` | Per `messaging.destination.name` |
| `tracegen.cache.operations` | Counter | `{operation}` | With `result` = `hit`/`miss` |

Two design notes worth stating explicitly.

**Cache hit rate is a counter pair, not a ratio gauge.** Ratios do not aggregate: averaging a hit-rate
gauge across pods gives an answer that is wrong in a way nobody notices. Emit hits and misses and let
the query divide.

**`-no-consumers` is the standout scenario.** With consumers disabled, `tracegen.messaging.queue.depth`
should climb without bound while the trace graph continues to look busy and healthy. That is a story
traces alone can only imply, and it is the clearest demonstration of why a metrics signal earns its
place next to them.

---

## Layer 2: GenAI metrics

Bounded, cheap, and directly aligned with the GenAI semantic conventions the AI scenarios already follow
on the trace side.

| Metric | Instrument | Unit | Attributes |
|---|---|---|---|
| `gen_ai.client.token.usage` | Histogram | `{token}` | `gen_ai.operation.name`, `gen_ai.provider.name`, `gen_ai.request.model`, `gen_ai.token.type` (`input`/`output`) |
| `gen_ai.client.operation.duration` | Histogram | `s` | Same, plus `error.type` where applicable |

Token counts already exist as `gen_ai.usage.input_tokens` / `output_tokens` span attributes, so this is
a projection of data Snowglobe already produces rather than new simulation. Cardinality is naturally
small: 9 chat models across 5 systems and 4 operations.

---

## Layer 3: an emission oracle, off by default

Snowglobe occupies a position no real producer does: it knows the correct answer. When a scenario runs,
it knows exactly how many spans it created, for which service, with which outcome.

A `tracegen.spans.emitted` counter, incremented in a `SpanProcessor.OnEnd` hook, makes that knowledge
queryable. Diffing it against whatever the receiving pipeline reports answers one specific question:
**did every span that was sent get counted, exactly once?**

That is a delivery-integrity check, not a duplicate RED metric. Because it compares **totals** rather
than histogram shapes, it needs no bucket-for-bucket alignment with anything downstream, which keeps
Snowglobe fully conformant while still serving as the oracle.

Gated behind `-metrics-verify`, default off. An oracle nobody queries is just cardinality.

---

## Layer 0: RED metrics from spans

A `SpanProcessor.OnEnd` hook derives `http.server.request.duration` and friends from finished spans,
with **zero changes to the ~3,000 lines of scenario code** and with traces and metrics agreeing by
construction: the p99 in the metric is the same span you can click into.

The attribute set must be an explicit allowlist built at the instrument, never a copy of the span's
attributes. Spans carry `user.id`, `order_id`, and `session_id`. Those are cardinality bombs and must
never reach a metric.

---

## Cardinality budget

**This is the binding constraint, and it deserves more care than the metric list.**

Metric backends commonly bill per active series, and an OTLP push path typically has no cardinality
control anywhere between the producer and the bill. Nothing downstream will protect a careless label.

The series count for the RED layer is roughly:

```
services x routes x methods x statuses    (instance id off)
pods     x routes x methods x statuses    (instance id on)
```

At `-complexity heavy` that is 28 services or 59 pods against the same per-service dimensions, so
enabling `service.instance.id` multiplies the RED series count by roughly the average pod-per-service
count. Against a real deployment where a service commonly has one or two instances, Snowglobe would be an
outlier by an order of magnitude, across however many grids are running.

Therefore:

- **`service.instance.id` is omitted from metric attributes by default.** Opt in with
  `-metrics-instance-id` when per-pod resolution is actually the thing being demonstrated. This is a
  deliberate divergence from the trace and log resources, which keep it.
- **`http.route` must always be templated.** A raw path mints one series per path.
- **Explicit histogram boundaries are mandatory.** The SDK's default 17-boundary layout is retained per
  active series, which turns a cardinality problem into a memory problem.
- **Measure before shipping.** The estimate above is arithmetic, not an observation. Run it against a
  real endpoint and count.

---

## Aggregation temporality

`-metrics-temporality delta|cumulative`, defaulting to cumulative (the Go SDK default).

This flag is not gold-plating. Prometheus-backed stores are cumulative-native: `rate()` and `increase()`
compute last-minus-first with counter-reset detection, so delta samples yield zeros and phantom reset
spikes rather than an error. Meanwhile some OTLP ingest guidance asks producers for delta and tells
cumulative producers to insert a `cumulativetodelta` processor.

Which behaviour a given endpoint actually implements is frequently undocumented. A controllable producer
that can emit either shape on demand is the cheapest way to settle it empirically, which is a genuinely
useful thing for this tool to be.

---

## Implementation sketch

Mirror `newProvider()`: build a `MeterProvider` per pod alongside the existing tracer and logger, sharing
the same resource so all three signals join on `service.name`.

```go
exporter, err := otlpmetricgrpc.New(ctx, opts...)          // new direct dependency
reader := sdkmetric.NewPeriodicReader(exporter,
    sdkmetric.WithInterval(metricsInterval))
mp := sdkmetric.NewMeterProvider(
    sdkmetric.WithReader(reader),
    sdkmetric.WithResource(res),                            // same resource as the tracer
    sdkmetric.WithView(explicitBucketsForDurations()),
)
```

`go.opentelemetry.io/otel/sdk/metric` v1.43.0 already resolves as an indirect dependency; only
`exporters/otlp/otlpmetric/otlpmetricgrpc` needs adding to `go.mod` as a direct requirement.

Per-pod meter providers mean each export request carries exactly one resource, matching how the trace
providers already behave.

---

## Flags

| Flag | Default | Purpose |
|---|---|---|
| `-no-metrics` | off (metrics on) | Disable metric emission, mirroring `-no-logs` |
| `-metrics-interval` | `15s` | `PeriodicReader` flush interval |
| `-metrics-temporality` | `cumulative` | `delta` or `cumulative` |
| `-metrics-instance-id` | off | Add `service.instance.id` to metric attributes |
| `-metrics-verify` | off | Emit the layer 3 emission oracle |

---

## Open questions

1. **Metrics default on, or opt in?** On-by-default matches `-no-logs`, but any deployment pinned by
   digest starts emitting metrics the moment the image is bumped. That should be an intentional roll,
   not a surprise, particularly against a preview or metered ingest path.
2. **Should infrastructure metrics be internally consistent with the traces?** Making
   `system.cpu.utilization` rise when a pod's scenarios are busy is more work than sampling a random
   walk, and considerably more convincing.
3. **What is the real series count?** Measure it against a live endpoint before enabling anything by
   default.
