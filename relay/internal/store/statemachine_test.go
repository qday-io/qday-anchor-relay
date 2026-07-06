package store

import (
	"testing"
	"time"
)

// Full transition matrix (spec §6.1). Anything not listed here is forbidden.
func TestCanTransition_AllowedMatrix(t *testing.T) {
	allowed := []struct{ from, to Status }{
		{StatusPending, StatusAbelSubmitted},
		{StatusAbelSubmitted, StatusAbelConfirmed},
		{StatusAbelConfirmed, StatusContractRecorded},
		{StatusContractRecorded, StatusDone},
		// 冪等推進：已在鏈上/已記錄時允許跳段推進（規則 6.1-2 / 6.1-3）
		{StatusAbelSubmitted, StatusPending}, // mempool 消失回退重送
		{StatusPending, StatusFailed},
		{StatusAbelSubmitted, StatusFailed},
		{StatusAbelConfirmed, StatusFailed},
		{StatusContractRecorded, StatusFailed},
	}
	for _, tr := range allowed {
		if !CanTransition(tr.from, tr.to) {
			t.Errorf("CanTransition(%s, %s) = false, want true", tr.from, tr.to)
		}
	}
}

func TestCanTransition_ForbiddenSamples(t *testing.T) {
	forbidden := []struct{ from, to Status }{
		{StatusPending, StatusAbelConfirmed}, // 不可跳過 submit
		{StatusPending, StatusContractRecorded},
		{StatusPending, StatusDone},
		{StatusAbelSubmitted, StatusContractRecorded},
		{StatusAbelSubmitted, StatusDone},
		{StatusAbelConfirmed, StatusDone},
		{StatusAbelConfirmed, StatusPending},
		{StatusAbelConfirmed, StatusAbelSubmitted},
		{StatusDone, StatusPending}, // 終態不可離開
		{StatusDone, StatusFailed},
		{StatusFailed, StatusPending},
		{StatusFailed, StatusDone},
		{StatusContractRecorded, StatusPending},
	}
	for _, tr := range forbidden {
		if CanTransition(tr.from, tr.to) {
			t.Errorf("CanTransition(%s, %s) = true, want false", tr.from, tr.to)
		}
	}
}

func TestIsTerminal(t *testing.T) {
	for _, s := range []Status{StatusDone, StatusFailed} {
		if !IsTerminal(s) {
			t.Errorf("IsTerminal(%s) = false, want true", s)
		}
	}
	for _, s := range []Status{StatusPending, StatusAbelSubmitted, StatusAbelConfirmed, StatusContractRecorded} {
		if IsTerminal(s) {
			t.Errorf("IsTerminal(%s) = true, want false", s)
		}
	}
}

func TestParseStatus(t *testing.T) {
	for _, s := range AllStatuses {
		got, err := ParseStatus(string(s))
		if err != nil || got != s {
			t.Errorf("ParseStatus(%q) = %v, %v", s, got, err)
		}
	}
	if _, err := ParseStatus("BOGUS"); err == nil {
		t.Error("ParseStatus(BOGUS) expected error")
	}
}

// Backoff: base * 2^retryCount, capped at maxBackoff.
func TestRetryBackoff(t *testing.T) {
	base := 30 * time.Second
	cases := []struct {
		retryCount int
		want       time.Duration
	}{
		{0, 30 * time.Second},
		{1, 60 * time.Second},
		{2, 120 * time.Second},
		{4, 480 * time.Second},
		{20, maxBackoff}, // capped
	}
	for _, c := range cases {
		if got := RetryBackoff(base, c.retryCount); got != c.want {
			t.Errorf("RetryBackoff(%v, %d) = %v, want %v", base, c.retryCount, got, c.want)
		}
	}
}
