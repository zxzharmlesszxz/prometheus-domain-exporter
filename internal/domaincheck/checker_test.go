package domaincheck

import (
	"context"
	"errors"
	"fmt"
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
		Domains: []string{"example.com"},
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

	checker := NewChecker([]string{"example.com"}, 0)

	if checker.Lookup == nil {
		t.Fatal("NewChecker().Lookup = nil, want RDAP lookup")
	}
	if checker.LookupTimeout != DefaultLookupTimeout {
		t.Fatalf("NewChecker().LookupTimeout = %v, want %v", checker.LookupTimeout, DefaultLookupTimeout)
	}
	if len(checker.Domains) != 1 || checker.Domains[0] != "example.com" {
		t.Fatalf("NewChecker().Domains = %v, want example.com", checker.Domains)
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
		{0, DefaultLookupTimeout},
		{-1, DefaultLookupTimeout},
		{-10 * time.Second, DefaultLookupTimeout},
		{5 * time.Second, 5 * time.Second},
		{DefaultLookupTimeout, DefaultLookupTimeout},
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

	checker := NewChecker([]string{"example.com"}, 5*time.Second)

	if checker.LookupTimeout != 5*time.Second {
		t.Fatalf("LookupTimeout = %v, want 5s", checker.LookupTimeout)
	}
}

func TestSnapshotWithPartialFailure(t *testing.T) {
	t.Parallel()

	now := time.Unix(1_700_000_000, 0)
	expiration := now.Add(30 * 24 * time.Hour)
	snapshot := Checker{
		Domains: []string{"fails.example", "succeeds.example"},
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

func TestCheckerMarksDomainLookupFailures(t *testing.T) {
	t.Parallel()

	now := time.Unix(1_700_000_000, 0)
	snapshot := Checker{
		Domains: []string{"example.com"},
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
