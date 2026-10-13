package main

import (
	"testing"

	apitrace "go.opentelemetry.io/otel/trace"
)

// A CLIENT span names what it called. Without the attribute that identifies the callee, a backend can only
// key the dependency on the bare system or peer name, so the grid grows a second node for the same thing:
//
//   - postgresql spans (db-spans): db.namespace is Conditionally Required "if available", and the generator
//     always knows which database a flow uses. The grid runs several postgresql databases, so without it a
//     call keys as plain "postgresql", beside "postgresql-orders_db" and the rest. Systems the grid runs as
//     a single instance (redis, the search and vector stores) do not split, so they are not checked here.
//   - outbound calls that name a peer (service.peer.name): server.address is Required on an HTTP client
//     span (http-spans), and server.port is Conditionally Required once server.address is set. A bare
//     single-label peer name such as "sendgrid" cannot be told apart from an internal service, so a
//     backend that never sees it answer reads it as missing.

func TestClientSpansIdentifyTheirCallee(t *testing.T) {
	spans := recordedSpans(t)

	var dbClients, peerClients int
	violations := map[string]int{}
	for _, s := range spans {
		if s.SpanKind() != apitrace.SpanKindClient {
			continue
		}
		if sys, ok := attrOf(s, "db.system.name"); ok && sys.AsString() == "postgresql" {
			dbClients++
			for _, m := range missing(s, "db.namespace") {
				violations["CLIENT "+s.Name()+" missing "+m]++
			}
		}
		if _, ok := attrOf(s, "service.peer.name"); ok {
			peerClients++
			for _, m := range missing(s, "server.address", "server.port") {
				violations["CLIENT "+s.Name()+" missing "+m+" (service.peer.name set)"]++
			}
		}
	}
	if dbClients == 0 || peerClients == 0 {
		t.Fatalf("recorded %d database and %d peer-naming client spans: the harness is not driving the flows", dbClients, peerClients)
	}
	reportViolations(t, violations, "callee-identifying attribute")
}
