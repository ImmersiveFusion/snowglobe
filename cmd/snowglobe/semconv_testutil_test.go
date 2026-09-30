package main

import (
	"context"
	"os"
	"reflect"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

// Shared machinery for the semantic-convention conformance tests (SP-120 e-002/e-004).
//
// The tests drive every scenario flow through the real tracer() indirection (tracerPool) into a
// SpanRecorder, so they assert what the generator actually emits, not what the source text says.

var (
	flowRe   = regexp.MustCompile(`\{"[^"]+", (\w+Flow),`)
	tracerRe = regexp.MustCompile(`tracer\("([\w-]+)"\)`)
)

func allFlows() []func(context.Context) {
	return []func(context.Context){
		createOrderFlow, searchAndBrowseFlow, userLoginFlow, addToCartFlow, fullCheckoutFlow,
		healthCheckFlow, failedPaymentFlow, bulkNotificationFlow, inventorySyncFlow, scheduledReportFlow,
		stripeWebhookFlow, recommendationFlow, shippingUpdateFlow, returnRefundFlow, sagaCompensationFlow,
		timeoutCascadeFlow, ragSearchFlow, aiChatbotFlow, contentModerationFlow, multiStepAgentFlow,
	}
}

func flowName(f func(context.Context)) string {
	n := runtime.FuncForPC(reflect.ValueOf(f).Pointer()).Name()
	return n[strings.LastIndex(n, ".")+1:]
}

func readSource(t *testing.T, names ...string) string {
	t.Helper()
	var all string
	for _, n := range names {
		b, err := os.ReadFile(n)
		if err != nil {
			t.Fatalf("read %s: %v", n, err)
		}
		all += string(b)
	}
	return all
}

var (
	recordOnce  sync.Once
	recordSpans []sdktrace.ReadOnlySpan
)

// recordedSpans runs every flow 25 times against a recorder, once for the whole test binary (the flows
// sleep, so several tests each re-running 500 flows cost 22 s apiece), and returns the ended spans.
func recordedSpans(t *testing.T) []sdktrace.ReadOnlySpan {
	t.Helper()
	recordOnce.Do(func() { recordSpans = recordAllFlows(t, 25) })
	if recordSpans == nil {
		t.Fatal("recording failed in an earlier test")
	}
	return recordSpans
}

func recordAllFlows(t *testing.T, perFlow int) []sdktrace.ReadOnlySpan {
	t.Helper()

	// Every flow registered in main.go must be driven here, by NAME, so a new or swapped scenario cannot
	// escape this test.
	registered := map[string]bool{}
	for _, m := range flowRe.FindAllStringSubmatch(readSource(t, "main.go"), -1) {
		registered[m[1]] = true
	}
	driven := map[string]bool{}
	for _, f := range allFlows() {
		driven[flowName(f)] = true
	}
	if !reflect.DeepEqual(registered, driven) {
		var r, d []string
		for k := range registered {
			r = append(r, k)
		}
		for k := range driven {
			d = append(d, k)
		}
		sort.Strings(r)
		sort.Strings(d)
		t.Fatalf("main.go registers flows %v, test drives %v: update allFlows()", r, d)
	}

	rec := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSampler(sdktrace.AlwaysSample()), sdktrace.WithSpanProcessor(rec))
	defer func() { _ = tp.Shutdown(context.Background()) }()

	saved := tracerPool
	tracerPool = map[string][]trace.Tracer{}
	defer func() { tracerPool = saved }()
	for _, m := range tracerRe.FindAllStringSubmatch(readSource(t, "scenarios.go", "scenarios_ai.go", "helpers.go"), -1) {
		tracerPool[m[1]] = []trace.Tracer{tp.Tracer(m[1])}
	}

	savedErr, savedCons, savedLogs := errorMultiplier, consumersEnabled, logsDisabled
	errorMultiplier, consumersEnabled, logsDisabled = 1.0, true, true
	defer func() { errorMultiplier, consumersEnabled, logsDisabled = savedErr, savedCons, savedLogs }()

	var wg sync.WaitGroup
	for _, f := range allFlows() {
		for i := 0; i < perFlow; i++ {
			wg.Add(1)
			go func(f func(context.Context)) {
				defer wg.Done()
				f(context.Background())
			}(f)
		}
	}
	wg.Wait()
	return rec.Ended()
}

func attrOf(s sdktrace.ReadOnlySpan, key string) (attribute.Value, bool) {
	for _, a := range s.Attributes() {
		if string(a.Key) == key {
			return a.Value, true
		}
	}
	return attribute.Value{}, false
}

func missing(s sdktrace.ReadOnlySpan, keys ...string) []string {
	var out []string
	for _, k := range keys {
		if _, ok := attrOf(s, k); !ok {
			out = append(out, k)
		}
	}
	return out
}

func reportViolations(t *testing.T, violations map[string]int, what string) {
	t.Helper()
	if len(violations) == 0 {
		return
	}
	var keys []string
	for k := range violations {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		t.Errorf("%s (x%d)", k, violations[k])
	}
	t.Errorf("%d distinct %s violations", len(violations), what)
}
