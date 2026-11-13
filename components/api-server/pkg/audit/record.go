package audit

import (
	"bytes"
	"encoding/json"
	"strings"
	"time"
)

const timeLayout = "2006-01-02T15:04:05.000000Z"

// maxFieldBytes bounds client-controlled text so one request cannot bloat
// the trail.
const maxFieldBytes = 2048

// Line is the JSON form of an event (schema version 1). Field names and
// meanings do not change within a schema version; new fields may be added.
type Line struct {
	SchemaVersion int        `json:"schema_version"`
	EventID       string     `json:"event_id"`
	OccurredAt    string     `json:"occurred_at"`
	Actor         lineActor  `json:"actor"`
	Action        string     `json:"action"`
	Outcome       string     `json:"outcome"`
	Reason        string     `json:"reason"`
	Resource      lineRes    `json:"resource"`
	Transport     string     `json:"transport"`
	HTTP          lineHTTP   `json:"http"`
	GRPC          lineGRPC   `json:"grpc"`
	Source        lineSource `json:"source"`
	OperationID   string     `json:"operation_id"`
	TraceID       string     `json:"trace_id"`
	Details       mapField   `json:"details"`
}

type lineActor struct {
	Type     string `json:"type"`
	Username string `json:"username"`
	Subject  string `json:"subject"`
	Issuer   string `json:"issuer"`
	ClientID string `json:"client_id"`
}

type lineRes struct {
	Surface string   `json:"surface"`
	Kind    string   `json:"kind"`
	ID      string   `json:"id"`
	Parent  mapField `json:"parent"`
}

type lineHTTP struct {
	Method string `json:"method"`
	Path   string `json:"path"`
	Status int    `json:"status"`
}

type lineGRPC struct {
	Method string `json:"method"`
	Code   string `json:"code"`
}

type lineSource struct {
	IP        string `json:"ip"`
	UserAgent string `json:"user_agent"`
}

// mapField always encodes as an object with sorted keys, never null.
type mapField map[string]string

func (m mapField) MarshalJSON() ([]byte, error) {
	if m == nil {
		return []byte("{}"), nil
	}
	return marshal(map[string]string(m))
}

// NewLine converts an event to its schema version 1 form. Text is made valid
// UTF-8 without NUL bytes and bounded in length, so a client cannot make its
// own events unwritable or oversized.
func NewLine(e Event) *Line {
	return &Line{
		SchemaVersion: SchemaVersion,
		EventID:       e.EventID,
		OccurredAt:    e.OccurredAt.UTC().Truncate(time.Microsecond).Format(timeLayout),
		Actor: lineActor{
			Type: clean(e.Actor.Type), Username: clean(e.Actor.Username), Subject: clean(e.Actor.Subject),
			Issuer: clean(e.Actor.Issuer), ClientID: clean(e.Actor.ClientID),
		},
		Action:  clean(e.Action),
		Outcome: e.Outcome,
		Reason:  clean(e.Reason),
		Resource: lineRes{
			Surface: clean(e.Resource.Surface), Kind: clean(e.Resource.Kind),
			ID: clean(e.Resource.ID), Parent: cleanMap(e.Resource.Parent),
		},
		Transport:   e.Transport,
		HTTP:        lineHTTP{Method: clean(e.HTTP.Method), Path: clean(e.HTTP.Path), Status: e.HTTP.Status},
		GRPC:        lineGRPC{Method: clean(e.GRPC.Method), Code: e.GRPC.Code},
		Source:      lineSource{IP: clean(e.Source.IP), UserAgent: clean(e.Source.UserAgent)},
		OperationID: clean(e.OperationID),
		TraceID:     clean(e.TraceID),
		Details:     cleanMap(e.Details),
	}
}

// MarshalLine encodes an event as one JSON line, including the trailing newline.
func MarshalLine(e Event) ([]byte, error) {
	b, err := marshal(NewLine(e))
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}

func clean(s string) string {
	s = strings.ToValidUTF8(s, "�")
	s = strings.ReplaceAll(s, "\x00", "�")
	if len(s) > maxFieldBytes {
		s = strings.ToValidUTF8(s[:maxFieldBytes], "")
	}
	return s
}

func cleanMap(m map[string]string) mapField {
	if len(m) == 0 {
		return nil
	}
	out := make(mapField, len(m))
	for k, v := range m {
		out[clean(k)] = clean(v)
	}
	return out
}

// marshal encodes JSON without HTML escaping and without a trailing newline.
func marshal(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(buf.Bytes(), []byte("\n")), nil
}
