![A sealed glass dome enclosing a glowing city of services, three ribbons of light rising through it](.img/banner.jpg)

# Snowglobe

**A sealed world you can shake.**

One binary that generates a whole distributed system's telemetry: logs, traces and metrics from a 28-service topology with the shapes real systems actually make. Diamond dependencies, scatter-gather fan-out, sagas that compensate, timeouts that cascade. Point it at anything that speaks OTLP and watch it arrive.

No Docker Compose. No microservices to deploy. No 6GB stack to babysit. One executable, or a 5.7 MB container.

```bash
docker run --rm immersivefusion/snowglobe -insecure -endpoint host.docker.internal:4317
```

**A public good for the OpenTelemetry community.** Apache-2.0, no account, no sign-up, no lock-in. The output is plain OTLP, so it works in whatever you already run: Jaeger, Tempo, Grafana, SigNoz, an OpenTelemetry Collector, or spatial tools such as DeepCube, which are one consumer among many.

**How this differs from `telemetrygen`.** `telemetrygen` generates spans, and it is the official tool and good at that. Snowglobe generates a *system*: a topology with real dependency shapes, injectable failures, and AI agent spans under the OTel GenAI semantic conventions. **If you need load, use `telemetrygen`. If you need something that looks like a production incident, that is what this is for.**

Most trace generators die. Their repos go quiet, the SDKs move, and the thing stops building. This one is maintained, and that is the point of it.

**It has a sibling.** Snowglobe makes realistic telemetry out of nothing: **synthetic** OTel. [sos-beacon](https://github.com/ImmersiveFusion/sos-beacon) is the **organic** counterpart, a real workload doing a real job that emits real telemetry as a byproduct. Between them you can fill a backend with either kind.

![Snowglobe traces in DeepCube's 3D player](.img/screenshot.png)

## Why This Exists

Every existing trace generator falls into one of two categories:

1. **Flat span generators** (telemetrygen, tracepusher) - produce uniform, identical spans with no service topology
2. **Full demo apps** (OTel Astronomy Shop, Jaeger HotROD) - require Docker Compose with 15+ containers and ~6GB RAM

And none of them generate **AI agentic traces**. The LLM observability market has no standalone tool that combines traditional APM with LLM observability. Every specialized LLM tool (Langfuse, LangSmith, Helicone, Arize, Traceloop, Portkey, Galileo) tracks token usage, model costs, and agent tool calls - but none of them provide traditional distributed tracing.

This tool generates **topology-rich, failure-injectable traces from a single binary** - covering both traditional microservice flows AND AI agentic patterns with OTel GenAI semantic conventions. One binary proves that a platform can visualize both.

## Quick Start

```bash
# Download the latest release (or build from source)
go install github.com/ImmersiveFusion/snowglobe/cmd/snowglobe@latest

# Send to a local OTLP collector (Jaeger, Tempo, etc.)
snowglobe -insecure

# Send to a remote endpoint with auth headers
snowglobe -endpoint your-otlp-endpoint:443 -headers "api-key=YOUR_KEY"

# Or set headers via the standard OTel environment variable
export OTEL_EXPORTER_OTLP_HEADERS="api-key=YOUR_KEY"
snowglobe -endpoint your-otlp-endpoint:443
```

**Installed from an old path?** Releases up to v0.7.7 were published as `github.com/ImmersiveFusion/if-opentelemetry-tracegen`, and releases up to v0.8.0 as `github.com/ImmersiveFusion/opentelemetry-tracegen`. Pinned installs of those versions keep working, but `@latest` on either old path stops resolving from the next release onward, so update your command to the path above. The installed binary is now named `snowglobe` rather than `tracegen`.

> **See it in 3D** - Send traces to [DeepCube](https://deepcube.ai) (`snowglobe -endpoint otlp.deepcube.ai:443 -headers "api-key=YOUR_KEY"`, [how to get a key](https://docs.deepcube.ai/Getting-Started/Api-Key/)) to explore them as a 3D force-directed graph, drill into conventional trace waterfalls for detailed analysis, and get AI-assisted insights from [Tessa](https://deepcube.ai). For a ready-made example without any setup, try [Shoebox](https://github.com/ImmersiveFusion/shoebox) at [shoebox.deepcube.ai](https://shoebox.deepcube.ai) - paste a diagram of a system, break a call in it, and fire one request through.

## Live demo grids: see it running

![The seven demo grids streaming live into DeepCube's 3D player](.img/deepcube.gif)

Seven always-on demo grids stream live OpenTelemetry traces, logs and metrics into DeepCube's 3D player right now: a clean baseline, an AI-native app, a blended environment, phantom-service detection, an AI-outage, and a full incident. Each grid is this container, deployed declaratively via GitOps (Argo CD) in the Immersive Fusion cloud, multi-arch and distroless, one matrix row per grid, shipping to `otlp.deepcube.ai:443`.

**Just want to look?** The grids are streamed live on Twitch at [twitch.tv/deepcubelive](https://www.twitch.tv/deepcubelive), around the clock. No account, no install, nothing to sign up for. It is running whether or not you do anything, which is the easiest way to see what this generator's output actually looks like at the other end.

**See them in 3D:** the full experience is [DeepCube](https://docs.deepcube.ai/DC/3D/), the immersive 3D client: install it and open a grid to walk the live traces. On mobile or can't install right now? [DeepCube Web](https://docs.deepcube.ai/DC/Web/) runs the same grids in your browser at [portal.deepcube.ai](https://portal.deepcube.ai).

**[Where else does Snowglobe run?](WHERE-SNOWGLOBE-RUNS.md)**: a community board of deployments. Add yours.

## Features

### 28 Microservices

#### Traditional Services (20)

| Service | Pods | Role |
|---|---|---|
| web-frontend | 2 | Browser client, SPA |
| api-gateway | 3 | HTTP routing, auth |
| order-service | 3 | Order lifecycle |
| payment-service | 2 | Stripe integration |
| inventory-service | 2 | Stock management |
| notification-service | 2 | Event-driven notifications |
| user-service | 2 | Auth, profiles |
| cache-service | 3 | Redis cluster |
| search-service | 2 | Elasticsearch queries |
| scheduler-service | 1 | Cron jobs (singleton) |
| auth-service | 3 | JWT, webhook verification |
| recommendation-service | 2 | ML-based recommendations |
| cart-service | 2 | Shopping cart |
| product-service | 3 | Product catalog |
| shipping-service | 2 | Rates, labels, tracking |
| fraud-service | 2 | ML fraud scoring |
| email-service | 2 | SMTP relay (SendGrid) |
| tax-service | 1 | Tax calculation |
| analytics-service | 3 | Event tracking (Kafka) |
| config-service | 1 | Feature flags |

#### AI Services (8)

| Service | Pods | Role |
|---|---|---|
| llm-gateway | 3 | OpenAI API routing, token tracking |
| embedding-service | 2 | Text-to-vector operations |
| vector-db-service | 2 | Qdrant similarity search |
| ai-agent-service | 2 | Agent orchestration (plan/act/reflect) |
| content-moderation-service | 2 | Safety classifiers, PII detection |
| model-registry-service | 1 | Model versioning (singleton) |
| feature-store-service | 2 | ML feature serving |
| data-pipeline-service | 2 | Batch embedding, retraining |

All 59 pods are distributed across 5 AKS VMSS nodes (2 node pools) with realistic `service.instance.id` and `host.name` resource attributes.

### 40 Scenario Flows

#### Traditional Scenarios (15 original + 13 new)

| Scenario | Graph Shape | Key Pattern |
|---|---|---|
| **Create Order** | Long chain (8 services, 14+ spans) | Producer/consumer with queue delays |
| **Search & Browse** | Linear with cache | Elasticsearch + Redis |
| **User Login** | Branching (success/failure) | Auth with session creation |
| **Failed Payment** | Error chain | Stripe 402 + error propagation |
| **Bulk Notifications** | Fan-out (3-5 parallel) | Batch email processing |
| **Health Check** | Star topology (6 parallel) | Concurrent health pings |
| **Inventory Sync** | Fan-out + reindex | Parallel cache warming |
| **Scheduled Report** | Headless chain (no UI) | Cron job entry point |
| **Stripe Webhook** | Headless chain (no gateway) | External callback entry |
| **Recommendations** | Scatter-gather / bowtie | Fan-out to 3, gather, cache |
| **Add to Cart** | Cross-service with feature flags | Config service + analytics |
| **Full Checkout** | Monster chain (15 services) | Tax+shipping parallel, fraud ML |
| **Shipping Update** | Carrier webhook (headless) | External webhook + email relay |
| **Saga Compensation** | Forward chain + 4-way compensation fan-out | Payment retries + rollback |
| **Timeout Cascade** | Branching with circuit breaker | Stale cache fallback |
| **User Registration** | Linear with async branch | Email verification token, duplicate detection |
| **Product Review** | Write + async moderation | Optimistic write + background processing |
| **Return/Refund** | Parallel reverse flow (16-18 spans) | Parallel refund + restock, reverse money flow |
| **Wishlist + Price Alert** | Write-through with async | Write-through cache, async price monitoring |
| **Coupon Application** | Validation chain | Cart recalculation, validation branch |
| **Gift Card Purchase** | Payment splitting | Balance check, payment splitting |
| **Subscription Management** | Webhook-driven lifecycle | Stripe subscription, renewal webhook |
| **A/B Test Exposure** | Feature flag branch | Variant assignment, sticky session |
| **Rate Limiting** | Early termination (4-6 spans) | Redis sliding window, 429 response |
| **Admin Product CRUD** | Write-amplification fan-out | Cache + search reindex on write |
| **Order History** | Paginated read | Keyset pagination, cursor-based |
| **Support Ticket** | Cross-domain trace | SLA assignment, team routing |
| **Multi-Currency Checkout** | External API chain | FX rate API, cache hit ratio |

#### AI Agentic Scenarios (12)

| Scenario | Graph Shape | Key Pattern |
|---|---|---|
| **Semantic Search (RAG)** | Linear with 2 LLM calls (14-16 spans) | Embedding + vector search + LLM reranking |
| **AI Chatbot with Tool Use** | Double bowtie (18-22 spans) | Plan -> fan-out tool calls -> synthesize |
| **AI Content Moderation** | Parallel classifiers + 3-way branch (12-16 spans) | Safety/spam scoring, guardrail decisions |
| **Multi-Step Agent** | Iterative loop (28-40 spans) | Plan -> act -> reflect cycle (3-5 iterations) |
| **AI Customer Support** | Branching with escalation (16-20 spans) | Sentiment classification, intent detection |
| **AI Content Generation** | Linear with safety filter (12-15 spans) | Temperature-controlled generation, content safety |
| **Embedding Pipeline** | High fan-out batch (25-40 spans) | Batch chunking, parallel embedding, vector upsert |
| **Dynamic Pricing Agent** | Headless agent (14-18 spans) | Feature store lookup, autonomous price updates |
| **Fraud with Explainability** | Linear with LLM explanation (10-12 spans) | SHAP-style feature attribution via LLM |
| **Inventory Reorder Agent** | Autonomous agent (16-20 spans) | Demand forecast, autonomous purchase orders |
| **Model Retraining Pipeline** | Batch pipeline (14-18 spans) | ML training spans, model registry, quality gate |
| **Conversational Commerce** | Multi-turn session (10-14 spans/turn) | Growing context tokens, session continuity |

> **Note:** Failed Payment, Saga Compensation, Timeout Cascade, lost messages, and retry storms only activate when `-errors > 0`. AI error scenarios (rate limits, hallucinated tool calls, token budget exceeded, content filter blocks) also require `-errors > 0`.

### Correlated Logs

Every service emits OTel log records via OTLP alongside traces. Logs are automatically correlated with the active span context (trace_id, span_id), so your APM platform can link logs to the exact span that produced them.

- **ERROR** logs are emitted alongside every exception event (cache failures, DB errors, payment declines, LLM rate limits, agent failures)
- **WARN** logs fire on auth failures, content moderation flags, payment retries, and LLM fallbacks
- **INFO** logs cover request entry points, payment processing, fraud analysis, agent invocations, and iteration progress

Disable with `-no-logs` to emit traces only.

### Metrics

Every service emits OTel metrics via OTLP alongside traces and logs. They are **derived from the spans the scenarios already produce**, so metrics and traces agree by construction: the p99 in a histogram is the same span you can click into.

Metrics aggregate in process and flush on an interval, so unlike traces their volume is decoupled from `-level`. A `-level 10` run emits roughly 350 traces/second and the *same* metric load as `-level 1`.

| Metric | Type | What it shows |
|---|---|---|
| `http.server.request.duration` | Histogram (s) | RED metrics per route, method and status. Semconv names, seconds, spec bucket boundaries |
| `http.server.active_requests` | UpDownCounter | In-flight requests, rising and falling with real scenario concurrency |
| `system.cpu.utilization` | Gauge | Driven by each service's actual span volume, not a random walk |
| `system.memory.usage` | Gauge | Resident memory, drifting with load |
| `db.client.connection.count` | UpDownCounter | Pool occupancy split `used`/`idle`, tracking real database spans |
| `tracegen.messaging.queue.depth` | Gauge | Published minus consumed, per destination |
| `gen_ai.client.token.usage` | Histogram | Input and output tokens by model, system and operation |
| `gen_ai.client.operation.duration` | Histogram (s) | LLM call latency |

**The queue depth is the one to watch.** Producer spans increment it and consumer spans decrement it, so running with `-no-consumers` makes the backlog climb without bound while the trace graph still looks busy. That is a story traces alone can only imply.

**Naming.** Where OpenTelemetry defines a metric, Snowglobe uses its name and conforms to its contract, including seconds as the unit. Everything else lives under a `tracegen.` prefix rather than extending a semconv namespace. That prefix keeps the old name on purpose: a metric name is a wire contract, and renaming it to `snowglobe.` would break every dashboard and alert already keyed on it. Do not sweep it.

**Cardinality.** `service.instance.id` is deliberately **off** by default: it multiplies every series by the pod count (up to 59 at `-complexity heavy`), and metric backends commonly bill per active series with no cardinality control on an OTLP push path. Turn it on with `-metrics-instance-id` when per-pod resolution is the thing you are demonstrating. Metric attributes are an explicit allowlist, never a copy of the span's attributes, so user, session and order ids never reach a metric.

**Temporality.** `-metrics-temporality delta` switches from the default cumulative. Prometheus-backed stores are cumulative-native, while some OTLP ingest paths ask producers for delta, and which behavior a given endpoint implements is often undocumented. A producer that can emit either shape on demand is the cheapest way to settle that by experiment.

Disable with `-no-metrics` to emit traces and logs only.

### OTel GenAI Semantic Conventions

All AI scenarios emit spans following [OTel GenAI Semantic Conventions](https://opentelemetry.io/docs/specs/semconv/gen-ai/) and matching the exact span shapes produced by [Microsoft Semantic Kernel](https://learn.microsoft.com/en-us/semantic-kernel/concepts/enterprise-readiness/observability/) and [Microsoft Agent Framework](https://learn.microsoft.com/en-us/agent-framework/agents/observability).

**Span types:**

| Span Name Pattern | SpanKind | Example |
|---|---|---|
| `chat {model}` | CLIENT | `chat gpt-4o` |
| `embedding {model}` | CLIENT | `embedding text-embedding-3-small` |
| `invoke_agent {name}` | CLIENT | `invoke_agent CustomerSupportAgent` |
| `execute_tool {name}` | INTERNAL | `execute_tool get_order_status` |
| `{operation} {collection}` | CLIENT | `query product-embeddings` |

**Attributes on every LLM span:**

- `gen_ai.system` - LLM provider (e.g., `openai`)
- `gen_ai.request.model` / `gen_ai.response.model` - model requested and used
- `gen_ai.usage.input_tokens` / `gen_ai.usage.output_tokens` - token consumption
- `gen_ai.response.finish_reasons` - completion reason (`stop`, `tool_calls`, `length`, `content_filter`)
- `gen_ai.response.id` - unique response identifier
- `gen_ai.request.temperature`, `gen_ai.request.max_tokens` - request parameters

**Agent-specific attributes:**

- `gen_ai.agent.id` / `gen_ai.agent.name` / `gen_ai.agent.description` - agent identity
- `gen_ai.conversation.id` - session linking for multi-turn interactions
- `gen_ai.tool.name` / `gen_ai.tool.type` / `gen_ai.tool.call.id` - tool call tracking
- `gen_ai.data_source.id` - RAG data source identifier
- `gen_ai.request.embedding.dimensions` - embedding dimensions

These attributes match what every LLM observability tool on the market tracks - enabling direct comparison of visualization capabilities.

### Chaos & Failure Injection

| Feature | Description |
|---|---|
| **Lost messages** | 5% chance per queue hop that the consumer never fires - trace ends abruptly |
| **Dead consumer mode** | `-no-consumers` flag: producers fire, consumers never pick up |
| **Retry storms** | Payment retries 3x with exponential backoff before saga compensation |
| **Timeout cascades** | Search service times out, gateway returns 504, circuit breaker serves stale cache |
| **Saga compensation** | Payment fails after order+inventory committed - triggers 4-way parallel rollback |
| **LLM rate limits** | OpenAI 429 with token budget details, fallback to text search |
| **Hallucinated tool calls** | Agent requests non-existent tool, triggers error handling |
| **Token budget exceeded** | Agent exceeds iteration token limit, graceful degradation |
| **Content filter blocks** | Safety classifier blocks content, alternate flow triggered |
| **Tunable error rate** | `-errors 0` (none) to `-errors 10` (chaos) with realistic .NET stack traces |

### Realistic Details

The generated traces simulate a .NET-based e-commerce platform with AI capabilities. Stack traces and library names reflect the .NET ecosystem by design.

- **Stack traces**: Npgsql, StackExchange.Redis, Stripe SDK, Elasticsearch.Net, System.Net.Http, OpenAI SDK, Qdrant client
- **Database operations**: PostgreSQL INSERT/SELECT/UPDATE with semantic conventions
- **Cache operations**: Redis GET/SET/HSET/MSET/DEL with TTL and key attributes
- **Messaging**: RabbitMQ and Kafka with producer/consumer span kinds and queue delays
- **External APIs**: Stripe charges, SendGrid email, UPS shipping, OpenAI chat/embeddings
- **LLM operations**: Chat completions, embeddings, agent tool calls with token tracking
- **Vector search**: Qdrant similarity search with cosine distance, dimension validation
- **Agent orchestration**: Plan/act/reflect loops, tool dispatch, session management
- **Content moderation**: Safety classifiers, PII detection, guardrail enforcement
- **ML inference**: Fraud detection model scoring with feature counts
- **Feature flags**: Config service checks that gate behavior

## Usage

```
snowglobe [flags]

Flags:
  -endpoint string     OTLP gRPC endpoint host:port (default "localhost:4317")
  -headers string      OTLP headers as key=value pairs, comma-separated (or set OTEL_EXPORTER_OTLP_HEADERS)
  -complexity string   Topology complexity: light, normal, heavy (default "normal")
  -level int           Aggressiveness 1-10 (default 1)
  -errors int          Error rate 0-10 (default 0)
  -no-consumers        Disable all async consumers
  -no-ai-backends      Disable LLM/AI backends (AI spans emit errors)
  -ai-only             Only run AI agentic scenarios
  -no-logs             Disable OTel log record emission (traces only)
  -no-metrics          Disable OTel metric emission
  -metrics-interval    Metric export interval (default 15s)
  -metrics-temporality Metric aggregation temporality: cumulative or delta (default cumulative)
  -metrics-instance-id Add service.instance.id to metrics (per-pod resolution; multiplies series count)
  -metrics-verify      Emit tracegen.spans.emitted, the emission oracle
  -insecure            Use plaintext gRPC (no TLS) for local backends
  -log-level string    Console verbosity: silent, error, info, debug (default "info")
  -quiet               Errors only (alias for -log-level=error); silences the periodic "traces sent" heartbeat
```

> **Console verbosity.** `silent`/`error` suppress the periodic heartbeat. The startup banner ("what it's doing") and genuine errors/fatal exits always print to stderr at any level. You can also set it with the `TRACEGEN_LOG_LEVEL` env var (handy for containers). Precedence: `-quiet` > `-log-level` > `TRACEGEN_LOG_LEVEL` > default (`info`). The published container image defaults to `TRACEGEN_LOG_LEVEL=error` so it doesn't spam logs; the bare CLI stays `info`. Override with `-e TRACEGEN_LOG_LEVEL=info` (or `-log-level` / `-quiet`).

### Complexity Levels

| Complexity | Services | Pods | Scenarios | Best for |
|---|---|---|---|---|
| **light** | 10 core | ~20 (min replicas) | 6 | Clean demos, small graphs |
| **normal** | 20 traditional | ~40 | 16 | General testing, full e-commerce |
| **heavy** | 28 (+ AI) | 59 | 20 (of 40 defined) | Full topology with AI agentic flows |

**Light** includes only the e-commerce backbone: web-frontend, api-gateway, order-service, payment-service, inventory-service, user-service, cache-service, auth-service, product-service, and cart-service. Scenarios are limited to the core flows (Create Order, Search & Browse, User Login, Add to Cart, Full Checkout, Health Check).

**Normal** (default) adds all remaining traditional services and scenarios including chaos/failure modes.

**Heavy** adds all 8 AI services and 4 of the 12 AI agentic scenarios (RAG Search, AI Chatbot, Content Moderation, Multi-Step Agent).

### Aggressiveness Levels

| Level | Label | Rate |
|---|---|---|
| 1 | whisper | ~2 traces/s |
| 2 | gentle | ~3 traces/s |
| 3 | calm | ~3 traces/s |
| 4 | moderate | ~5 traces/s |
| 5 | steady | ~7 traces/s |
| 6 | brisk | ~15 traces/s |
| 7 | aggressive | ~21 traces/s |
| 8 | intense | ~40 traces/s |
| 9 | firehose | ~83 traces/s |
| 10 | SCREAM | ~350 traces/s |

### Examples

```bash
# Send to a local Jaeger/Tempo/Collector (default endpoint localhost:4317)
snowglobe -insecure

# Clean demo with minimal services - great for presentations
snowglobe -complexity light -level 1 -insecure

# Full e-commerce topology (default)
snowglobe -level 1 -insecure

# Everything including AI agentic scenarios
snowglobe -complexity heavy -level 3 -insecure

# Moderate load with normal error rates
snowglobe -level 5 -errors 5 -insecure

# Simulate dead consumers (messages pile up, consumers never fire)
snowglobe -level 3 -no-consumers -insecure

# AI scenarios only - great for LLM observability testing
snowglobe -level 3 -ai-only -insecure

# Simulate AI backend outage (LLM rate limits, timeouts)
snowglobe -level 5 -no-ai-backends -errors 5 -insecure

# Chaos mode - maximum load and errors
snowglobe -level 10 -errors 10 -insecure

# Send to a remote endpoint with authentication
snowglobe -endpoint otlp.example.com:443 -headers "api-key=YOUR_KEY"

# Multiple headers via environment variable
export OTEL_EXPORTER_OTLP_HEADERS="api-key=SECRET,x-team=platform"
snowglobe -endpoint otlp.example.com:443

# Send to DeepCube (3D trace visualization)
snowglobe -endpoint otlp.deepcube.ai:443 -headers "API-Key=YOUR_DEEPCUBE_KEY"
```

## How It Compares

| Capability | Snowglobe | OTel telemetrygen | OTel Astronomy Shop | Jaeger HotROD | k6 + xk6-tracing |
|---|:---:|:---:|:---:|:---:|:---:|
| Single binary, zero infra | **Yes** | 1 binary | 15+ containers, ~6GB | 4 containers | k6 + extension |
| Services | **28** | 1 | ~22 | 4 | User-defined |
| Pod instances | **59** | 0 | 1/svc | 0 | 0 |
| Scenario flows | **40** | 0 | ~10 | 1 | User-defined |
| AI agentic scenarios | **12** | No | No | No | No |
| OTel GenAI conventions | **Yes** | No | No | No | No |
| Agent tool call traces | **Yes** | No | No | No | No |
| RAG pipeline traces | **Yes** | No | No | No | No |
| Diamond dependencies | **Yes** | No | Implicit | No | No |
| Scatter-gather | **Yes** | No | No | No | No |
| Lost messages | **Yes** | No | No | No | No |
| Dead consumer mode | **Yes** | No | No | No | No |
| Saga compensation | **Yes** | No | No | No | No |
| Retry storms | **Yes** | No | No | No | No |
| Timeout cascade | **Yes** | No | No | No | No |
| LLM failure injection | **Yes** | No | No | No | No |
| Tunable error rate | **0-10** | No | Fixed | No | No |
| Tunable throughput | **2-350/s** | Rate flag | Locust | Fixed | k6 VUs |
| Headless flows (webhook/cron) | **3** | No | No | No | No |
| Startup time | **<1s** | <1s | 3-5 min | 30s | <5s |

## Compatible Backends

Works with any OTLP gRPC-compatible backend:

- Any OpenTelemetry Collector
- Datadog (with OTLP endpoint)
- DeepCube (3D visualization)
- Elastic APM
- Grafana Tempo
- Honeycomb
- Jaeger
- New Relic
- Splunk Observability

Listed alphabetically on purpose. DeepCube is one consumer of this tool among many, and the tool is not built to favour it.

The AI agentic traces are also compatible with LLM-specialized observability tools that accept OTel input:

- Langfuse (OTel-native since SDK v3)
- Arize Phoenix (OTel instrumentation)
- Traceloop / OpenLLMetry (built on OTel)

## Related Tools

Part of a small family of single-binary, zero-infra OTel tools, all Apache-2.0 and all usable without any Immersive Fusion account:

- **[sos-beacon](https://github.com/ImmersiveFusion/sos-beacon)** - The **organic** counterpart to this tool's synthetic output. A real workload doing a real job (surfacing people asking for help in public forums, for a human to answer), OTel-instrumented, so it emits genuine production telemetry as a byproduct. Where Snowglobe invents a system, sos-beacon *is* one.
- **[Shoebox](https://github.com/ImmersiveFusion/shoebox)** - The world you build, where Snowglobe is the world pre-made. Paste a Mermaid diagram of a system, break something in it, fire one request, and read what comes out. A snowglobe is sealed; in a shoebox you can open it up. [See both in 3D](https://shoebox.deepcube.ai).

## Building From Source

```bash
git clone https://github.com/ImmersiveFusion/snowglobe.git
cd snowglobe
go build -o snowglobe ./cmd/snowglobe
```

### Cross-compile

```bash
# Linux
GOOS=linux GOARCH=amd64 go build -o snowglobe ./cmd/snowglobe

# macOS (Apple Silicon)
GOOS=darwin GOARCH=arm64 go build -o snowglobe ./cmd/snowglobe

# Windows
GOOS=windows GOARCH=amd64 go build -o snowglobe.exe ./cmd/snowglobe
```

## Design Decisions

### Why AI Agentic Scenarios?

The LLM observability market is growing rapidly, but every specialized tool focuses exclusively on LLM workloads. No standalone LLM observability tool provides traditional APM capabilities. The only platforms addressing both are legacy APM giants (Datadog, New Relic, Dynatrace) adding LLM features to existing products.

This trace generator produces both traditional distributed traces AND AI agentic traces from the same binary - proving that a single platform can visualize both. The AI scenarios emit the exact same telemetry signals that Langfuse, LangSmith, Helicone, Arize, Traceloop, Portkey, and Galileo track.

### Why Microsoft Semantic Kernel / Agent Framework Alignment?

Microsoft's Semantic Kernel and Agent Framework are the most widely adopted .NET AI frameworks. Their OTel instrumentation emits exactly three span types: `invoke_agent {name}`, `chat {model}`, and `execute_tool {function}`. Our AI scenarios produce traces structurally identical to what a real Semantic Kernel / Agent Framework application would emit - so observability platforms can be tested against realistic .NET AI workloads.

### Why OTel GenAI Semantic Conventions?

The [OTel GenAI Semantic Conventions](https://opentelemetry.io/docs/specs/semconv/gen-ai/) are being adopted across the ecosystem. Langfuse SDK v3 is OTel-native, LangSmith added OTel support, Arize Phoenix uses OTel instrumentation, and Traceloop's OpenLLMetry conventions were adopted into the official OTel spec. Building on these conventions ensures the generated traces are compatible with every tool that adopts the standard.

### Sources

| Source | Decision Informed |
|--------|-------------------|
| [OTel GenAI Semantic Conventions](https://opentelemetry.io/docs/specs/semconv/gen-ai/) | Attribute names, span conventions, operation types |
| [OTel GenAI Agent Spans](https://opentelemetry.io/docs/specs/semconv/gen-ai/gen-ai-agent-spans/) | Agent span conventions: invoke_agent, execute_tool |
| [OTel GenAI Attribute Registry](https://opentelemetry.io/docs/specs/semconv/registry/attributes/gen-ai/) | Complete `gen_ai.*` attribute list with types |
| [MS Semantic Kernel Observability](https://learn.microsoft.com/en-us/semantic-kernel/concepts/enterprise-readiness/observability/) | Activity sources, metrics, `gen_ai` attribute usage |
| [MS Agent Framework Observability](https://learn.microsoft.com/en-us/agent-framework/agents/observability) | Production span shapes: `invoke_agent`, `chat`, `execute_tool` |
| LLM Observability Market Research (internal) | Market gap analysis, competitive positioning, feature parity requirements |
| [Langfuse OTel Integration](https://langfuse.com/integrations/native/opentelemetry) | OTel-native SDK v3, attribute expectations |
| [Traceloop OpenLLMetry](https://github.com/traceloop/openllmetry) | OTel GenAI conventions adopted into official spec |

## Connect

[Email](mailto:info@immersivefusion.com) |
[LinkedIn](https://www.linkedin.com/company/immersivefusion) |
[Discord](https://discord.gg/zevywnQp6K) |
[GitHub](https://github.com/immersivefusion) |
[Bluesky](https://bsky.app/profile/immersivefusion.bsky.social) |
[Twitter/X](https://twitter.com/immersivefusion) |
[YouTube](https://www.youtube.com/@immersivefusion) |
[Twitch](https://www.twitch.tv/immersivefusion)

## License

Apache License 2.0 - see [LICENSE](LICENSE) for details.

Copyright 2026 [ImmersiveFusion](https://immersivefusion.com)
