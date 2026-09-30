package main

import (
	"net/url"
	"testing"

	"go.opentelemetry.io/otel/sdk/trace"
	apitrace "go.opentelemetry.io/otel/trace"
)

// SP-120 E1. The HTTP semantic conventions (http-spans, semconv 1.44.0) make these Required on a SERVER
// span: http.request.method, url.path, url.scheme. server.address is Recommended, server.port is
// Conditionally Required when server.address is set, and both "are intended, whenever possible, to be the
// same on the client and server sides". A span is an HTTP span here when it carries http.request.method.

func TestHTTPServerSpansCarryRequiredAttributes(t *testing.T) {
	spans := recordedSpans(t)

	var httpServer int
	violations := map[string]int{}
	for _, s := range spans {
		if s.SpanKind() != apitrace.SpanKindServer {
			continue
		}
		if _, ok := attrOf(s, "http.request.method"); !ok {
			continue
		}
		httpServer++
		for _, m := range missing(s, "url.path", "url.scheme") {
			violations["SERVER "+s.Name()+" missing "+m]++
		}
		if _, ok := attrOf(s, "server.address"); ok {
			for _, m := range missing(s, "server.port") {
				violations["SERVER "+s.Name()+" missing "+m+" (server.address set)"]++
			}
		}
	}
	if httpServer == 0 {
		t.Fatal("recorded no HTTP server spans: the harness is not driving the flows")
	}
	reportViolations(t, violations, "Required-attribute")
}

// For EVERY HTTP client -> HTTP server pair the server reports what the client asked for: the same host,
// the same port, and the path of the client's url.full. Presence alone would let a wrong value through
// (the healthz server once said /healthz while its client requested /api/v2/healthz).
func TestHTTPServerSpanMirrorsCallingClientRequest(t *testing.T) {
	spans := recordedSpans(t)

	byID := map[apitrace.SpanID]trace.ReadOnlySpan{}
	for _, s := range spans {
		byID[s.SpanContext().SpanID()] = s
	}

	pairs := 0
	sawHealthz := false
	for _, s := range spans {
		if s.SpanKind() != apitrace.SpanKindServer {
			continue
		}
		if _, ok := attrOf(s, "http.request.method"); !ok {
			continue
		}
		parent, ok := byID[s.Parent().SpanID()]
		if !ok || parent.SpanKind() != apitrace.SpanKindClient {
			continue
		}
		if _, isHTTP := attrOf(parent, "http.request.method"); !isHTTP {
			continue
		}
		pairs++
		if s.Name() == "GET /healthz" {
			sawHealthz = true
		}

		full, _ := attrOf(parent, "url.full")
		u, err := url.Parse(full.AsString())
		if err != nil || u.Path == "" {
			t.Errorf("client %q has no parseable url.full %q", parent.Name(), full.AsString())
			continue
		}
		if got, ok := attrOf(s, "url.path"); !ok || got.AsString() != u.EscapedPath() {
			t.Errorf("server %q url.path=%q ok=%v, want the client's requested path %q", s.Name(), got.AsString(), ok, u.EscapedPath())
		}
		if got, ok := attrOf(s, "server.address"); !ok || got.AsString() != u.Hostname() {
			t.Errorf("server %q server.address=%q ok=%v, want the client's host %q", s.Name(), got.AsString(), ok, u.Hostname())
		}
		pAddr, _ := attrOf(parent, "server.address")
		if got, _ := attrOf(s, "server.address"); got.AsString() != pAddr.AsString() {
			t.Errorf("server %q server.address=%q differs from client attribute %q", s.Name(), got.AsString(), pAddr.AsString())
		}
		pPort, _ := attrOf(parent, "server.port")
		if got, ok := attrOf(s, "server.port"); !ok || got.AsInt64() != pPort.AsInt64() {
			t.Errorf("server %q server.port=%d ok=%v, want the client's %d", s.Name(), got.AsInt64(), ok, pPort.AsInt64())
		}
		if got, _ := attrOf(s, "url.scheme"); got.AsString() != u.Scheme {
			t.Errorf("server %q url.scheme=%q, client url.full scheme %q", s.Name(), got.AsString(), u.Scheme)
		}
	}
	if pairs == 0 {
		t.Fatal("no HTTP client -> HTTP server pair observed")
	}
	if !sawHealthz {
		t.Fatal("the health-check pair (the one that once disagreed) was not observed")
	}
}

// Nothing is invented: only the shop front door is ever a server host.
func TestNoInventedThirdPartyServers(t *testing.T) {
	for _, s := range recordedSpans(t) {
		if s.SpanKind() != apitrace.SpanKindServer {
			continue
		}
		if v, ok := attrOf(s, "server.address"); ok && v.AsString() != "shop.example.com" {
			t.Errorf("SERVER span %q carries server.address=%q: only the shop front door is served here", s.Name(), v.AsString())
		}
	}
}
