package audit

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// TableName is the audit event table.
const TableName = "trex_audit_events"

// CursorTableName stores the last exported chain_seq per sink.
const CursorTableName = "trex_audit_export_cursors"

// GenesisHash is the prev_hash of the first event in a chain.
var GenesisHash = strings.Repeat("0", 64)

// KeyIDNone is the key_id of events hashed without an HMAC key.
const KeyIDNone = "none"

const hashDomain = "trex-audit-v1\n"

const timeLayout = "2006-01-02T15:04:05.000000Z"

// row is the database form of an Event.
type row struct {
	IngestSeq      int64     `gorm:"column:ingest_seq;primaryKey;autoIncrement"`
	EventID        string    `gorm:"column:event_id"`
	OccurredAt     time.Time `gorm:"column:occurred_at"`
	ActorType      string    `gorm:"column:actor_type"`
	ActorUsername  string    `gorm:"column:actor_username"`
	ActorSubject   string    `gorm:"column:actor_subject"`
	ActorIssuer    string    `gorm:"column:actor_issuer"`
	ActorClientID  string    `gorm:"column:actor_client_id"`
	Action         string    `gorm:"column:action"`
	Outcome        string    `gorm:"column:outcome"`
	Reason         string    `gorm:"column:reason"`
	Transport      string    `gorm:"column:transport"`
	Surface        string    `gorm:"column:surface"`
	ResourceKind   string    `gorm:"column:resource_kind"`
	ResourceID     string    `gorm:"column:resource_id"`
	ResourceParent string    `gorm:"column:resource_parent"`
	HTTPMethod     string    `gorm:"column:http_method"`
	HTTPPath       string    `gorm:"column:http_path"`
	HTTPStatus     int       `gorm:"column:http_status"`
	GRPCMethod     string    `gorm:"column:grpc_method"`
	GRPCCode       string    `gorm:"column:grpc_code"`
	SourceIP       string    `gorm:"column:source_ip"`
	UserAgent      string    `gorm:"column:user_agent"`
	OperationID    string    `gorm:"column:operation_id"`
	TraceID        string    `gorm:"column:trace_id"`
	Details        string    `gorm:"column:details"`

	ChainSeq *int64     `gorm:"column:chain_seq"`
	PrevHash *string    `gorm:"column:prev_hash"`
	Hash     *string    `gorm:"column:hash"`
	KeyID    *string    `gorm:"column:key_id"`
	SealedAt *time.Time `gorm:"column:sealed_at"`
}

func (row) TableName() string { return TableName }

func toRow(e Event) (*row, error) {
	parent, err := encodeMap(e.Resource.Parent)
	if err != nil {
		return nil, fmt.Errorf("encoding resource parent: %w", err)
	}
	details, err := encodeMap(e.Details)
	if err != nil {
		return nil, fmt.Errorf("encoding details: %w", err)
	}
	return sanitize(&row{
		EventID:        e.EventID,
		OccurredAt:     e.OccurredAt.UTC().Truncate(time.Microsecond),
		ActorType:      e.Actor.Type,
		ActorUsername:  e.Actor.Username,
		ActorSubject:   e.Actor.Subject,
		ActorIssuer:    e.Actor.Issuer,
		ActorClientID:  e.Actor.ClientID,
		Action:         e.Action,
		Outcome:        e.Outcome,
		Reason:         e.Reason,
		Transport:      e.Transport,
		Surface:        e.Resource.Surface,
		ResourceKind:   e.Resource.Kind,
		ResourceID:     e.Resource.ID,
		ResourceParent: parent,
		HTTPMethod:     e.HTTP.Method,
		HTTPPath:       e.HTTP.Path,
		HTTPStatus:     e.HTTP.Status,
		GRPCMethod:     e.GRPC.Method,
		GRPCCode:       e.GRPC.Code,
		SourceIP:       e.Source.IP,
		UserAgent:      e.Source.UserAgent,
		OperationID:    e.OperationID,
		TraceID:        e.TraceID,
		Details:        details,
	}), nil
}

// maxFieldBytes bounds client-controlled text so one request cannot bloat the trail.
const maxFieldBytes = 2048

// clean makes s valid UTF-8 without NUL bytes, which PostgreSQL text rejects,
// and bounds its length. A client must not be able to make its own events fail
// to store by sending odd bytes.
func clean(s string) string {
	s = strings.ToValidUTF8(s, "\uFFFD")
	s = strings.ReplaceAll(s, "\x00", "\uFFFD")
	if len(s) > maxFieldBytes {
		s = strings.ToValidUTF8(s[:maxFieldBytes], "")
	}
	return s
}

func sanitize(r *row) *row {
	for _, f := range []*string{
		&r.ActorUsername, &r.ActorSubject, &r.ActorIssuer, &r.ActorClientID, &r.Action, &r.Reason,
		&r.Surface, &r.ResourceKind, &r.ResourceID, &r.HTTPMethod, &r.HTTPPath,
		&r.GRPCMethod, &r.SourceIP, &r.UserAgent, &r.OperationID, &r.TraceID,
	} {
		*f = clean(*f)
	}
	return r
}

// encodeMap stores a string map as JSON with sorted keys, or "" when empty.
func encodeMap(m map[string]string) (string, error) {
	if len(m) == 0 {
		return "", nil
	}
	cleaned := make(map[string]string, len(m))
	for k, v := range m {
		cleaned[clean(k)] = clean(v)
	}
	b, err := marshalCanonical(cleaned)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func decodeMap(s string) (map[string]string, error) {
	m := map[string]string{}
	if s == "" {
		return m, nil
	}
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		return nil, err
	}
	return m, nil
}

// Line is one exported event (schema version 1). Its JSON form, without
// prev_hash and hash, is the canonical encoding that the chain hashes.
// Field order is part of the format.
type Line struct {
	SchemaVersion int        `json:"schema_version"`
	ChainSeq      int64      `json:"chain_seq"`
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
	KeyID         string     `json:"key_id"`
	PrevHash      string     `json:"prev_hash,omitempty"`
	Hash          string     `json:"hash,omitempty"`
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

// mapField always encodes as an object, never null.
type mapField map[string]string

func (m mapField) MarshalJSON() ([]byte, error) {
	if m == nil {
		return []byte("{}"), nil
	}
	return marshalCanonical(map[string]string(m))
}

// lineFor builds the canonical line of a row with the given seal values.
// prev and hash are left empty for hashing.
func lineFor(r *row, chainSeq int64, keyID string) (*Line, error) {
	parent, err := decodeMap(r.ResourceParent)
	if err != nil {
		return nil, fmt.Errorf("decoding resource parent of event %s: %w", r.EventID, err)
	}
	details, err := decodeMap(r.Details)
	if err != nil {
		return nil, fmt.Errorf("decoding details of event %s: %w", r.EventID, err)
	}
	return &Line{
		SchemaVersion: SchemaVersion,
		ChainSeq:      chainSeq,
		EventID:       r.EventID,
		OccurredAt:    r.OccurredAt.UTC().Format(timeLayout),
		Actor: lineActor{
			Type: r.ActorType, Username: r.ActorUsername, Subject: r.ActorSubject,
			Issuer: r.ActorIssuer, ClientID: r.ActorClientID,
		},
		Action:      r.Action,
		Outcome:     r.Outcome,
		Reason:      r.Reason,
		Resource:    lineRes{Surface: r.Surface, Kind: r.ResourceKind, ID: r.ResourceID, Parent: parent},
		Transport:   r.Transport,
		HTTP:        lineHTTP{Method: r.HTTPMethod, Path: r.HTTPPath, Status: r.HTTPStatus},
		GRPC:        lineGRPC{Method: r.GRPCMethod, Code: r.GRPCCode},
		Source:      lineSource{IP: r.SourceIP, UserAgent: r.UserAgent},
		OperationID: r.OperationID,
		TraceID:     r.TraceID,
		Details:     details,
		KeyID:       keyID,
	}, nil
}

// Canonical returns the bytes the chain hashes: the line without prev_hash and hash.
func (l *Line) Canonical() ([]byte, error) {
	c := *l
	c.PrevHash = ""
	c.Hash = ""
	return marshalCanonical(&c)
}

// ComputeHash returns the chain hash of a line given the previous hash.
// With a nil key it is SHA-256; otherwise HMAC-SHA256.
func ComputeHash(key []byte, prevHash string, l *Line) (string, error) {
	canonical, err := l.Canonical()
	if err != nil {
		return "", err
	}
	var input bytes.Buffer
	input.WriteString(hashDomain)
	input.WriteString(prevHash)
	input.WriteString("\n")
	input.Write(canonical)
	if key == nil {
		sum := sha256.Sum256(input.Bytes())
		return hex.EncodeToString(sum[:]), nil
	}
	mac := hmac.New(sha256.New, key)
	mac.Write(input.Bytes())
	return hex.EncodeToString(mac.Sum(nil)), nil
}

// KeyID identifies a key without revealing it.
func KeyID(key []byte) string {
	if key == nil {
		return KeyIDNone
	}
	sum := sha256.Sum256(key)
	return hex.EncodeToString(sum[:])[:16]
}

// marshalCanonical encodes JSON without HTML escaping and without a trailing newline.
func marshalCanonical(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(buf.Bytes(), []byte("\n")), nil
}
