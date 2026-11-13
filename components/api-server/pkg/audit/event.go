// Package audit captures the TRex audit trail (SEC-004). TRex captures every
// REST and gRPC request that changes state or that authentication or
// authorization refuses, and hands each event to the environment's Recorder.
// Applications add their own events with Record.
package audit

import "time"

// SchemaVersion is the version of the exported JSON-lines schema.
const SchemaVersion = 1

// Outcomes.
const (
	OutcomeSuccess         = "success"
	OutcomeFailure         = "failure"
	OutcomeDenied          = "denied"
	OutcomeUnauthenticated = "unauthenticated"
	OutcomeError           = "error"
)

// Actor types.
const (
	ActorUser      = "user"
	ActorSystem    = "system"
	ActorAnonymous = "anonymous"
)

// Transports.
const (
	TransportHTTP     = "http"
	TransportGRPC     = "grpc"
	TransportInternal = "internal"
)

// Actor is who performed the action.
type Actor struct {
	Type     string
	Username string
	Subject  string
	Issuer   string
	ClientID string
}

// Resource is what the action targeted, as in SEC-002.
type Resource struct {
	Surface string
	Kind    string
	ID      string
	Parent  map[string]string
}

// HTTPInfo describes a REST request. Status is set for non-success outcomes.
type HTTPInfo struct {
	Method string
	Path   string
	Status int
}

// GRPCInfo describes a gRPC call. Code is set for non-success outcomes.
type GRPCInfo struct {
	Method string
	Code   string
}

// Source is where the request came from.
type Source struct {
	IP        string
	UserAgent string
}

// Event is one audit entry. Zero fields that Record can derive from the
// context (EventID, OccurredAt, OperationID, TraceID, Actor, Transport) are
// filled in when it is recorded.
//
// An Event MUST NOT carry request or response bodies, query strings,
// credentials, or tokens. Details holds short, non-secret strings only.
type Event struct {
	EventID     string
	OccurredAt  time.Time
	Actor       Actor
	Action      string
	Outcome     string
	Reason      string
	Resource    Resource
	Transport   string
	HTTP        HTTPInfo
	GRPC        GRPCInfo
	Source      Source
	OperationID string
	TraceID     string
	Details     map[string]string
}

// isRead reports whether an action only reads state.
func isRead(action string) bool {
	switch action {
	case "get", "list", "watch":
		return true
	}
	return false
}
