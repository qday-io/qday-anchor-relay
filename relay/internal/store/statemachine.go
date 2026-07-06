package store

import (
	"fmt"
	"time"
)

// Status is the anchor_jobs.status value (spec §6.1).
type Status string

const (
	StatusPending          Status = "PENDING"
	StatusAbelSubmitted    Status = "ABEL_SUBMITTED"
	StatusAbelConfirmed    Status = "ABEL_CONFIRMED"
	StatusContractRecorded Status = "CONTRACT_RECORDED"
	StatusDone             Status = "DONE"
	StatusFailed           Status = "FAILED"
)

var AllStatuses = []Status{
	StatusPending, StatusAbelSubmitted, StatusAbelConfirmed,
	StatusContractRecorded, StatusDone, StatusFailed,
}

// allowedTransitions is the complete legal transition set. The happy path is
// PENDING → ABEL_SUBMITTED → ABEL_CONFIRMED → CONTRACT_RECORDED → DONE.
// ABEL_SUBMITTED → PENDING covers rule 6.1-2's "tx vanished from mempool,
// roll back and resubmit". Every non-terminal state may fail.
var allowedTransitions = map[Status]map[Status]bool{
	StatusPending: {
		StatusAbelSubmitted: true,
		StatusFailed:        true,
	},
	StatusAbelSubmitted: {
		StatusAbelConfirmed: true,
		StatusPending:       true,
		StatusFailed:        true,
	},
	StatusAbelConfirmed: {
		StatusContractRecorded: true,
		StatusFailed:           true,
	},
	StatusContractRecorded: {
		StatusDone:   true,
		StatusFailed: true,
	},
}

// CanTransition reports whether from → to is a legal state-machine move.
func CanTransition(from, to Status) bool {
	return allowedTransitions[from][to]
}

// IsTerminal reports whether jobs in s are never picked up again.
func IsTerminal(s Status) bool {
	return s == StatusDone || s == StatusFailed
}

// ParseStatus validates a raw DB string into a Status.
func ParseStatus(raw string) (Status, error) {
	for _, s := range AllStatuses {
		if string(s) == raw {
			return s, nil
		}
	}
	return "", fmt.Errorf("store: unknown job status %q", raw)
}

// maxBackoff caps exponential retry growth (2^n explodes quickly).
const maxBackoff = 30 * time.Minute

// RetryBackoff returns base * 2^retryCount, capped at maxBackoff.
func RetryBackoff(base time.Duration, retryCount int) time.Duration {
	if base <= 0 {
		base = time.Second
	}
	d := base
	for i := 0; i < retryCount; i++ {
		d *= 2
		if d >= maxBackoff {
			return maxBackoff
		}
	}
	return d
}
