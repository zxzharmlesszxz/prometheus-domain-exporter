package domaincheck

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

type fakeExpirationLookup struct {
	expirations map[string]time.Time
	errors      map[string]error
}

func (l fakeExpirationLookup) LookupExpiration(_ context.Context, domain string) (time.Time, error) {
	if err := l.errors[domain]; err != nil {
		return time.Time{}, err
	}
	return l.expirations[domain], nil
}

func TestCheckerCollectsDomainExpirations(t *testing.T) {
	t.Parallel()

	now := time.Unix(1_700_000_000, 0)
	expiration := now.Add(30 * 24 * time.Hour)
	snapshot := Checker{
		Targets: []string{"example.com"},
		Lookup: fakeExpirationLookup{
			expirations: map[string]time.Time{"example.com": expiration},
		},
	}.Snapshot(context.Background(), now)

	if !snapshot.Success {
		t.Fatalf("Snapshot().Success = false, want true: %v", snapshot.Err)
	}
	if len(snapshot.Domains) != 1 {
		t.Fatalf("Snapshot().Domains length = %d, want 1", len(snapshot.Domains))
	}
	got := snapshot.Domains[0]
	if got.Name != "example.com" || !got.Expiration.Equal(expiration) || !got.Success {
		t.Fatalf("Snapshot().Domains[0] = %+v, want successful example.com result", got)
	}
}

func TestNewCheckerSetsRDAPLookupAndDefaults(t *testing.T) {
	t.Parallel()

	checker := NewChecker([]string{"example.com"}, 0, 0)

	if checker.Lookup == nil {
		t.Fatal("NewChecker().Lookup = nil, want RDAP lookup")
	}
	if checker.Timeout != 0 {
		t.Fatalf("NewChecker().Timeout = %v, want 0 (normalized in Snapshot)", checker.Timeout)
	}
	if checker.MaxConcurrentTargets != DefaultMaxConcurrentTargets {
		t.Fatalf("NewChecker().MaxConcurrentTargets = %d, want %d", checker.MaxConcurrentTargets, DefaultMaxConcurrentTargets)
	}
	if len(checker.Targets) != 1 || checker.Targets[0] != "example.com" {
		t.Fatalf("NewChecker().Targets = %v, want example.com", checker.Targets)
	}
}

func TestCheckerWithoutDomainsSucceeds(t *testing.T) {
	t.Parallel()

	now := time.Unix(1_700_000_000, 0)
	snapshot := Checker{}.Snapshot(context.Background(), now)

	if !snapshot.Success {
		t.Fatalf("Snapshot().Success = false, want true: %v", snapshot.Err)
	}
	if len(snapshot.Domains) != 0 {
		t.Fatalf("Snapshot().Domains length = %d, want 0", len(snapshot.Domains))
	}
}

func TestNormalizeLookupTimeout(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input time.Duration
		want  time.Duration
	}{
		{0, DefaultTimeout},
		{-1, DefaultTimeout},
		{-10 * time.Second, DefaultTimeout},
		{5 * time.Second, 5 * time.Second},
		{DefaultTimeout, DefaultTimeout},
		{30 * time.Second, 30 * time.Second},
	}
	for _, tt := range tests {
		if got := normalizeLookupTimeout(tt.input); got != tt.want {
			t.Errorf("normalizeLookupTimeout(%v) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestNewCheckerPreservesExplicitTimeout(t *testing.T) {
	t.Parallel()

	checker := NewChecker([]string{"example.com"}, 5*time.Second, 2)

	if checker.Timeout != 5*time.Second {
		t.Fatalf("Timeout = %v, want 5s", checker.Timeout)
	}
	if checker.MaxConcurrentTargets != 2 {
		t.Fatalf("MaxConcurrentTargets = %d, want 2", checker.MaxConcurrentTargets)
	}
}

func TestSnapshotWithPartialFailure(t *testing.T) {
	t.Parallel()

	now := time.Unix(1_700_000_000, 0)
	expiration := now.Add(30 * 24 * time.Hour)
	snapshot := Checker{
		Targets: []string{"fails.example", "succeeds.example"},
		Lookup: fakeExpirationLookup{
			expirations: map[string]time.Time{"succeeds.example": expiration},
			errors:      map[string]error{"fails.example": fmt.Errorf("rdap unavailable")},
		},
	}.Snapshot(context.Background(), now)

	if snapshot.Success {
		t.Fatal("Snapshot().Success = true, want false")
	}
	if len(snapshot.Domains) != 2 {
		t.Fatalf("Snapshot().Domains length = %d, want 2", len(snapshot.Domains))
	}
	if snapshot.Domains[0].Success || snapshot.Domains[0].Name != "fails.example" {
		t.Fatal("first domain should have failed")
	}
	if !snapshot.Domains[1].Success || snapshot.Domains[1].Name != "succeeds.example" {
		t.Fatal("second domain should have succeeded")
	}
}

func TestCheckerHonorsMaxConcurrent(t *testing.T) {
	t.Parallel()

	now := time.Unix(1_700_000_000, 0)
	expiration := now.Add(30 * 24 * time.Hour)
	var mu sync.Mutex
	var concurrency int
	var maxSeen int

	snapshot := Checker{
		Targets: []string{"a.example", "b.example", "c.example"},
		Lookup: fakeConcurrentLookup{
			expiration:  expiration,
			mu:          &mu,
			concurrency: &concurrency,
			maxSeen:     &maxSeen,
		},
		MaxConcurrentTargets: 2,
	}.Snapshot(context.Background(), now)

	if !snapshot.Success {
		t.Fatalf("Snapshot().Success = false, want true: %v", snapshot.Err)
	}
	if len(snapshot.Domains) != 3 {
		t.Fatalf("Snapshot().Domains length = %d, want 3", len(snapshot.Domains))
	}
	if maxSeen > 2 {
		t.Fatalf("max concurrent lookups = %d, want <= 2", maxSeen)
	}
}

type fakeConcurrentLookup struct {
	expiration  time.Time
	mu          *sync.Mutex
	concurrency *int
	maxSeen     *int
}

func (l fakeConcurrentLookup) LookupExpiration(_ context.Context, _ string) (time.Time, error) {
	l.mu.Lock()
	*l.concurrency++
	if *l.concurrency > *l.maxSeen {
		*l.maxSeen = *l.concurrency
	}
	l.mu.Unlock()

	time.Sleep(10 * time.Millisecond)

	l.mu.Lock()
	*l.concurrency--
	l.mu.Unlock()

	return l.expiration, nil
}

func TestSnapshotRespectsContextCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	snapshot := Checker{
		Targets: []string{"a.example", "b.example"},
		Lookup: fakeExpirationLookup{
			expirations: map[string]time.Time{
				"a.example": time.Now(),
				"b.example": time.Now(),
			},
		},
	}.Snapshot(ctx, time.Now())

	if snapshot.Success {
		t.Fatal("Snapshot().Success = true, want false")
	}
	if !errors.Is(snapshot.Err, context.Canceled) {
		t.Fatalf("Snapshot().Err = %v, want context.Canceled", snapshot.Err)
	}
	if len(snapshot.Domains) != 0 {
		t.Fatalf("Snapshot().Domains length = %d, want 0", len(snapshot.Domains))
	}
}

func TestCheckerMarksDomainLookupFailures(t *testing.T) {
	t.Parallel()

	now := time.Unix(1_700_000_000, 0)
	snapshot := Checker{
		Targets: []string{"example.com"},
		Lookup: fakeExpirationLookup{
			errors: map[string]error{"example.com": errors.New("rdap unavailable")},
		},
	}.Snapshot(context.Background(), now)

	if snapshot.Success {
		t.Fatal("Snapshot().Success = true, want false")
	}
	if snapshot.Err == nil {
		t.Fatal("Snapshot().Err = nil, want error")
	}
	if len(snapshot.Domains) != 1 || snapshot.Domains[0].Success {
		t.Fatalf("Snapshot().Domains = %+v, want failed domain result", snapshot.Domains)
	}
}
