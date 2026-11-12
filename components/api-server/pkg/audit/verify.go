package audit

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// VerifyOptions configures Verify.
type VerifyOptions struct {
	// ExpectHead is "<chain_seq>:<hash>", typically the newest event a SIEM
	// holds. The chain must contain that exact event.
	ExpectHead string
	// MaxUnsealedAge reports unsealed events older than this. Zero disables the check.
	MaxUnsealedAge time.Duration
	// MaxProblems stops the walk after this many problems. Zero means 20.
	MaxProblems int
}

// VerifyResult is the outcome of Verify.
type VerifyResult struct {
	Checked       int64
	HeadSeq       int64
	HeadHash      string
	Unsealed      int64
	StaleUnsealed int64
	Problems      []string
}

// OK reports whether the chain is intact.
func (r *VerifyResult) OK() bool { return len(r.Problems) == 0 }

// Verify walks the chain in chain_seq order and checks contiguity, links,
// hashes, and key IDs against the configured key.
func (s *Service) Verify(ctx context.Context, opts VerifyOptions) (*VerifyResult, error) {
	if s == nil || s.sessions == nil {
		return nil, ErrNoSessionFactory
	}
	maxProblems := opts.MaxProblems
	if maxProblems == 0 {
		maxProblems = 20
	}
	var expectSeq int64
	var expectHash string
	if opts.ExpectHead != "" {
		seqStr, hash, ok := strings.Cut(opts.ExpectHead, ":")
		seq, err := strconv.ParseInt(seqStr, 10, 64)
		if !ok || err != nil || seq < 1 || hash == "" {
			return nil, fmt.Errorf("--expect-head must be <chain_seq>:<hash>, got %q", opts.ExpectHead)
		}
		expectSeq, expectHash = seq, hash
	}

	res := &VerifyResult{HeadHash: GenesisHash}
	problem := func(format string, args ...any) bool {
		res.Problems = append(res.Problems, fmt.Sprintf(format, args...))
		return len(res.Problems) >= maxProblems
	}

	session := s.sessions.New(detach(ctx))
	prev := GenesisHash
	var next int64 = 1
	expectSeen := false
	keyMismatchReported := false
	batch := s.batchSize
	if batch == 0 {
		batch = 500
	}
walk:
	for {
		var rows []row
		if err := session.Where("chain_seq >= ?", next).Order("chain_seq").Limit(batch).Find(&rows).Error; err != nil {
			return nil, fmt.Errorf("reading chain: %w", err)
		}
		if len(rows) == 0 {
			break
		}
		for i := range rows {
			r := &rows[i]
			seq := *r.ChainSeq
			if seq != next {
				if problem("gap: chain_seq %d to %d missing before event %s", next, seq-1, r.EventID) {
					break walk
				}
			}
			if r.PrevHash == nil || r.Hash == nil || r.KeyID == nil {
				if problem("chain_seq %d: incomplete seal", seq) {
					break walk
				}
				next = seq + 1
				continue
			}
			if *r.PrevHash != prev {
				if problem("chain_seq %d: prev_hash does not match the hash of chain_seq %d", seq, seq-1) {
					break walk
				}
			}
			if *r.KeyID != s.keyID && !keyMismatchReported {
				keyMismatchReported = true
				if problem("chain_seq %d: key_id %s does not match the configured key %s", seq, *r.KeyID, s.keyID) {
					break walk
				}
			}
			line, err := lineFor(r, seq, *r.KeyID)
			if err != nil {
				if problem("chain_seq %d: %v", seq, err) {
					break walk
				}
			} else if *r.KeyID == s.keyID {
				want, err := ComputeHash(s.key, *r.PrevHash, line)
				if err != nil {
					return nil, err
				}
				if want != *r.Hash {
					if problem("chain_seq %d: hash mismatch (event %s was changed)", seq, r.EventID) {
						break walk
					}
				}
			}
			if expectSeq == seq {
				expectSeen = true
				if *r.Hash != expectHash {
					if problem("chain_seq %d: hash differs from the expected head", seq) {
						break walk
					}
				}
			}
			res.Checked++
			res.HeadSeq = seq
			res.HeadHash = *r.Hash
			prev = *r.Hash
			next = seq + 1
		}
	}
	if expectSeq != 0 && !expectSeen && len(res.Problems) < maxProblems {
		problem("expected head chain_seq %d is missing; the chain ends at %d (truncated tail?)", expectSeq, res.HeadSeq)
	}

	var outside int64
	if err := session.Table(TableName).Where("chain_seq < 1").Count(&outside).Error; err != nil {
		return nil, fmt.Errorf("counting events outside the chain: %w", err)
	}
	if outside > 0 {
		res.Problems = append(res.Problems, fmt.Sprintf("%d events carry a chain_seq below 1 and are hidden from the chain", outside))
	}

	if err := session.Table(TableName).Where("chain_seq IS NULL").Count(&res.Unsealed).Error; err != nil {
		return nil, fmt.Errorf("counting unsealed events: %w", err)
	}
	if opts.MaxUnsealedAge > 0 {
		cutoff := time.Now().Add(-opts.MaxUnsealedAge)
		if err := session.Table(TableName).Where("chain_seq IS NULL AND occurred_at < ?", cutoff).Count(&res.StaleUnsealed).Error; err != nil {
			return nil, fmt.Errorf("counting stale unsealed events: %w", err)
		}
		if res.StaleUnsealed > 0 {
			res.Problems = append(res.Problems, fmt.Sprintf("%d events unsealed for longer than %s", res.StaleUnsealed, opts.MaxUnsealedAge))
		}
	}
	return res, nil
}
