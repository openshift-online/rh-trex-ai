package audit

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	dbContext "github.com/openshift-online/rh-trex-ai/components/api-server/pkg/db/db_context"
)

// sealLockKey is the PostgreSQL advisory lock that serializes sealing and
// export across replicas ("trexaudt" as a big-endian int64).
const sealLockKey int64 = 0x7472657861756474

// PassResult reports what one sealing pass did.
type PassResult struct {
	Acquired bool
	Sealed   int
	Exported int
	Unsealed int64
}

// SealOnce runs one sealing pass and then one export pass. Each pass holds
// the advisory lock in its own transaction; when another replica holds it,
// the pass is skipped. Sealing commits before export, so a sink only ever
// receives committed seals, and a failing sink does not stop sealing.
func (s *Service) SealOnce(ctx context.Context) (PassResult, error) {
	var res PassResult
	if !s.Enabled() {
		return res, nil
	}
	if s.sessions == nil {
		return res, ErrNoSessionFactory
	}
	// Keep cancellation and deadlines; drop any request transaction.
	ctx = dbContext.WithTransaction(ctx, nil)

	err := s.locked(ctx, func(tx *gorm.DB) error {
		res.Acquired = true
		sealed, err := s.sealPending(tx)
		if err != nil {
			return err
		}
		res.Sealed = sealed
		if err := tx.Table(TableName).Where("chain_seq IS NULL").Count(&res.Unsealed).Error; err != nil {
			return fmt.Errorf("counting unsealed events: %w", err)
		}
		return nil
	})
	if err != nil {
		metricSealFailures.Inc()
		return res, err
	}
	if res.Acquired {
		metricSealed.Add(float64(res.Sealed))
		metricUnsealed.Set(float64(res.Unsealed))
	}

	err = s.locked(ctx, func(tx *gorm.DB) error {
		res.Acquired = true
		exported, err := s.export(tx)
		res.Exported = exported
		return err
	})
	if err != nil {
		metricSealFailures.Inc()
		return res, err
	}
	metricExported.Add(float64(res.Exported))
	return res, nil
}

// locked runs fn in a transaction that holds the seal lock, or does nothing
// when another replica holds it.
func (s *Service) locked(ctx context.Context, fn func(tx *gorm.DB) error) error {
	return s.sessions.New(ctx).Transaction(func(tx *gorm.DB) error {
		var acquired bool
		if err := tx.Raw("SELECT pg_try_advisory_xact_lock(?)", sealLockKey).Scan(&acquired).Error; err != nil {
			return fmt.Errorf("acquiring seal lock: %w", err)
		}
		if !acquired {
			return nil
		}
		return fn(tx)
	})
}

// head returns the newest sealed chain_seq and hash, or 0 and the genesis hash.
func head(tx *gorm.DB) (int64, string, error) {
	var last row
	err := tx.Where("chain_seq IS NOT NULL").Order("chain_seq DESC").Limit(1).Take(&last).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, GenesisHash, nil
	}
	if err != nil {
		return 0, "", fmt.Errorf("reading chain head: %w", err)
	}
	if last.ChainSeq == nil || last.Hash == nil {
		return 0, "", errors.New("chain head has no seal")
	}
	return *last.ChainSeq, *last.Hash, nil
}

func (s *Service) sealPending(tx *gorm.DB) (int, error) {
	seq, prev, err := head(tx)
	if err != nil {
		return 0, err
	}
	var pending []row
	if err := tx.Where("chain_seq IS NULL").Order("ingest_seq").Limit(s.batchSize).Find(&pending).Error; err != nil {
		return 0, fmt.Errorf("reading unsealed events: %w", err)
	}
	now := time.Now().UTC()
	for i := range pending {
		r := &pending[i]
		seq++
		line, err := lineFor(r, seq, s.keyID)
		if err != nil {
			return 0, err
		}
		hash, err := ComputeHash(s.key, prev, line)
		if err != nil {
			return 0, err
		}
		result := tx.Table(TableName).
			Where("ingest_seq = ? AND chain_seq IS NULL", r.IngestSeq).
			Updates(map[string]interface{}{
				"chain_seq": seq,
				"prev_hash": prev,
				"hash":      hash,
				"key_id":    s.keyID,
				"sealed_at": now,
			})
		if result.Error != nil {
			return 0, fmt.Errorf("sealing event %s: %w", r.EventID, result.Error)
		}
		if result.RowsAffected != 1 {
			return 0, fmt.Errorf("sealing event %s: row changed concurrently", r.EventID)
		}
		prev = hash
	}
	return len(pending), nil
}

type cursor struct {
	Sink         string    `gorm:"column:sink;primaryKey"`
	LastChainSeq int64     `gorm:"column:last_chain_seq"`
	UpdatedAt    time.Time `gorm:"column:updated_at"`
}

func (cursor) TableName() string { return CursorTableName }

// export writes sealed events after the sink's cursor and advances it.
func (s *Service) export(tx *gorm.DB) (int, error) {
	if s.sink == nil {
		return 0, nil
	}
	var c cursor
	err := tx.Where("sink = ?", s.sinkName).Take(&c).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, fmt.Errorf("reading export cursor: %w", err)
	}
	var rows []row
	if err := tx.Where("chain_seq > ?", c.LastChainSeq).Order("chain_seq").Limit(s.batchSize).Find(&rows).Error; err != nil {
		return 0, fmt.Errorf("reading events to export: %w", err)
	}
	if len(rows) == 0 {
		return 0, nil
	}
	lines := make([][]byte, 0, len(rows))
	for i := range rows {
		b, err := exportLine(&rows[i])
		if err != nil {
			return 0, err
		}
		lines = append(lines, b)
	}
	if err := s.sink.WriteLines(lines); err != nil {
		return 0, fmt.Errorf("writing to audit sink: %w", err)
	}
	last := *rows[len(rows)-1].ChainSeq
	err = tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "sink"}},
		DoUpdates: clause.AssignmentColumns([]string{"last_chain_seq", "updated_at"}),
	}).Create(&cursor{Sink: s.sinkName, LastChainSeq: last, UpdatedAt: time.Now().UTC()}).Error
	if err != nil {
		return 0, fmt.Errorf("saving export cursor: %w", err)
	}
	return len(rows), nil
}

// exportLine renders a sealed row as one JSON line with a trailing newline.
func exportLine(r *row) ([]byte, error) {
	if r.ChainSeq == nil || r.Hash == nil || r.PrevHash == nil || r.KeyID == nil {
		return nil, fmt.Errorf("event %s is not sealed", r.EventID)
	}
	l, err := lineFor(r, *r.ChainSeq, *r.KeyID)
	if err != nil {
		return nil, err
	}
	l.PrevHash = *r.PrevHash
	l.Hash = *r.Hash
	b, err := marshalCanonical(l)
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}

// Start runs sealing passes every seal interval until Stop is called. It
// returns immediately when the service is disabled.
func (s *Service) Start() {
	if !s.Enabled() {
		return
	}
	s.runMu.Lock()
	if s.stop != nil {
		s.runMu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.stop = cancel
	s.stopped = make(chan struct{})
	s.runMu.Unlock()

	go func() {
		defer close(s.stopped)
		ticker := time.NewTicker(s.sealInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.drain(ctx)
			}
		}
	}()
}

// drain runs passes until a pass seals and exports less than a full batch.
func (s *Service) drain(ctx context.Context) {
	for ctx.Err() == nil {
		res, err := s.SealOnce(ctx)
		if err != nil {
			slog.ErrorContext(ctx, "Audit sealing pass failed", "error", err)
			return
		}
		if !res.Acquired || (res.Sealed < s.batchSize && res.Exported < s.batchSize) {
			return
		}
	}
}

// Stop ends the sealing loop, runs a final pass so recent events are sealed
// and exported, and closes the sink.
func (s *Service) Stop() error {
	if !s.Enabled() {
		return nil
	}
	s.runMu.Lock()
	stop, stopped := s.stop, s.stopped
	s.stop = nil
	s.runMu.Unlock()
	if stop != nil {
		stop()
		<-stopped
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	s.drain(ctx)
	if s.sink != nil {
		return s.sink.Close()
	}
	return nil
}
