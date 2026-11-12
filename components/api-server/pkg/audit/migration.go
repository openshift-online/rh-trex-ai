package audit

import (
	"time"

	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"

	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/db"
)

func init() {
	db.RegisterMigration(migration())
}

// guardSQL makes trex_audit_events append-only. The only UPDATE allowed is the
// one that seals an unsealed row and changes nothing but the seal columns.
const guardSQL = `
CREATE OR REPLACE FUNCTION trex_audit_events_guard() RETURNS trigger AS $$
BEGIN
	IF TG_OP = 'DELETE' OR TG_OP = 'TRUNCATE' THEN
		RAISE EXCEPTION 'trex_audit_events is append-only';
	END IF;
	IF OLD.chain_seq IS NOT NULL THEN
		RAISE EXCEPTION 'sealed audit events are immutable';
	END IF;
	IF NEW.chain_seq IS NULL OR NEW.hash IS NULL OR NEW.prev_hash IS NULL OR NEW.key_id IS NULL OR NEW.sealed_at IS NULL THEN
		RAISE EXCEPTION 'an audit event can only be updated to seal it';
	END IF;
	IF NEW.chain_seq <> COALESCE((SELECT max(chain_seq) FROM trex_audit_events), 0) + 1 THEN
		RAISE EXCEPTION 'an audit event must be sealed at the next chain position';
	END IF;
	IF NEW.prev_hash IS DISTINCT FROM COALESCE(
		(SELECT hash FROM trex_audit_events WHERE chain_seq = NEW.chain_seq - 1),
		repeat('0', 64)) THEN
		RAISE EXCEPTION 'an audit event must link to the previous chain hash';
	END IF;
	IF (to_jsonb(NEW) - ARRAY['chain_seq','prev_hash','hash','key_id','sealed_at'])
		IS DISTINCT FROM (to_jsonb(OLD) - ARRAY['chain_seq','prev_hash','hash','key_id','sealed_at']) THEN
		RAISE EXCEPTION 'audit event content is immutable';
	END IF;
	RETURN NEW;
END
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trex_audit_events_guard_row ON trex_audit_events;
CREATE TRIGGER trex_audit_events_guard_row
	BEFORE UPDATE OR DELETE ON trex_audit_events
	FOR EACH ROW EXECUTE FUNCTION trex_audit_events_guard();

DROP TRIGGER IF EXISTS trex_audit_events_guard_truncate ON trex_audit_events;
CREATE TRIGGER trex_audit_events_guard_truncate
	BEFORE TRUNCATE ON trex_audit_events
	FOR EACH STATEMENT EXECUTE FUNCTION trex_audit_events_guard();

CREATE INDEX IF NOT EXISTS idx_trex_audit_events_unsealed
	ON trex_audit_events (ingest_seq) WHERE chain_seq IS NULL;
`

func migration() *gormigrate.Migration {
	// Local copies of the schema (DA-002). The event table deliberately does
	// not embed db.Model: audit rows have no updated_at and no soft delete.
	type TrexAuditEvent struct {
		IngestSeq      int64     `gorm:"column:ingest_seq;primaryKey;autoIncrement"`
		EventID        string    `gorm:"column:event_id;not null;uniqueIndex"`
		OccurredAt     time.Time `gorm:"column:occurred_at;not null;index"`
		ActorType      string    `gorm:"column:actor_type;not null"`
		ActorUsername  string    `gorm:"column:actor_username;not null;index"`
		ActorSubject   string    `gorm:"column:actor_subject;not null"`
		ActorIssuer    string    `gorm:"column:actor_issuer;not null"`
		ActorClientID  string    `gorm:"column:actor_client_id;not null"`
		Action         string    `gorm:"column:action;not null"`
		Outcome        string    `gorm:"column:outcome;not null"`
		Reason         string    `gorm:"column:reason;not null"`
		Transport      string    `gorm:"column:transport;not null"`
		Surface        string    `gorm:"column:surface;not null"`
		ResourceKind   string    `gorm:"column:resource_kind;not null;index:idx_trex_audit_events_resource"`
		ResourceID     string    `gorm:"column:resource_id;not null;index:idx_trex_audit_events_resource"`
		ResourceParent string    `gorm:"column:resource_parent;not null"`
		HTTPMethod     string    `gorm:"column:http_method;not null"`
		HTTPPath       string    `gorm:"column:http_path;not null"`
		HTTPStatus     int       `gorm:"column:http_status;not null"`
		GRPCMethod     string    `gorm:"column:grpc_method;not null"`
		GRPCCode       string    `gorm:"column:grpc_code;not null"`
		SourceIP       string    `gorm:"column:source_ip;not null"`
		UserAgent      string    `gorm:"column:user_agent;not null"`
		OperationID    string    `gorm:"column:operation_id;not null"`
		TraceID        string    `gorm:"column:trace_id;not null"`
		Details        string    `gorm:"column:details;not null"`

		ChainSeq *int64     `gorm:"column:chain_seq;uniqueIndex"`
		PrevHash *string    `gorm:"column:prev_hash"`
		Hash     *string    `gorm:"column:hash"`
		KeyID    *string    `gorm:"column:key_id"`
		SealedAt *time.Time `gorm:"column:sealed_at"`
	}
	type TrexAuditExportCursor struct {
		Sink         string    `gorm:"column:sink;primaryKey"`
		LastChainSeq int64     `gorm:"column:last_chain_seq;not null"`
		UpdatedAt    time.Time `gorm:"column:updated_at;not null"`
	}

	return &gormigrate.Migration{
		ID: "2026111212001944",
		Migrate: func(tx *gorm.DB) error {
			if err := tx.Table(TableName).AutoMigrate(&TrexAuditEvent{}); err != nil {
				return err
			}
			if err := tx.Table(CursorTableName).AutoMigrate(&TrexAuditExportCursor{}); err != nil {
				return err
			}
			return tx.Exec(guardSQL).Error
		},
		Rollback: func(tx *gorm.DB) error {
			if err := tx.Migrator().DropTable(CursorTableName); err != nil {
				return err
			}
			if err := tx.Migrator().DropTable(TableName); err != nil {
				return err
			}
			return tx.Exec("DROP FUNCTION IF EXISTS trex_audit_events_guard()").Error
		},
	}
}
