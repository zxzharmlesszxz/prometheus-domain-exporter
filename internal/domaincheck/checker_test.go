package domaincheck

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type countingExpirationLookup struct {
	expiration time.Time
	err        error
	calls      atomic.Int64
}

func (l *countingExpirationLookup) LookupExpiration(context.Context, string) (time.Time, bool, error) {
	l.calls.Add(1)
	if l.err != nil {
		return time.Time{}, false, l.err
	}
	return l.expiration, true, nil
}

type fakeExpirationLookup struct {
	expirations map[string]time.Time
	verified    map[string]bool
	errors      map[string]error
}

func (l fakeExpirationLookup) LookupExpiration(_ context.Context, domain string) (time.Time, bool, error) {
	if err := l.errors[domain]; err != nil {
		return time.Time{}, false, err
	}
	verified := true
	if l.verified != nil {
		verified = l.verified[domain]
	}
	return l.expirations[domain], verified, nil
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

type fakeSourcedExpirationLookup struct {
	expiration time.Time
	source     string
}

func (l fakeSourcedExpirationLookup) LookupExpiration(context.Context, string) (time.Time, bool, error) {
	return l.expiration, true, nil
}

func (l fakeSourcedExpirationLookup) LookupExpirationSource(context.Context, string) (time.Time, bool, string, error) {
	return l.expiration, true, l.source, nil
}

func TestCheckerRecordsLookupSource(t *testing.T) {
	t.Parallel()

	now := time.Unix(1_700_000_000, 0)
	snapshot := Checker{
		Targets: []string{"example.ws"},
		Lookup: fakeSourcedExpirationLookup{
			expiration: now.Add(24 * time.Hour),
			source:     SourceWHOIS,
		},
	}.Snapshot(context.Background(), now)

	if !snapshot.Success {
		t.Fatalf("Snapshot().Success = false, want true: %v", snapshot.Err)
	}
	if len(snapshot.Domains) != 1 || snapshot.Domains[0].Source != SourceWHOIS {
		t.Fatalf("Snapshot().Domains = %+v, want WHOIS source", snapshot.Domains)
	}
}

func TestNewCheckerSetsRDAPLookupAndDefaults(t *testing.T) {
	t.Parallel()

	checker := NewChecker([]string{"example.com"}, 0, 0)

	if checker.Lookup == nil {
		t.Fatal("NewChecker().Lookup = nil, want registration lookup")
	}
	if checker.Timeout != DefaultTimeout {
		t.Fatalf("NewChecker().Timeout = %v, want %v", checker.Timeout, DefaultTimeout)
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

func TestNormalizeMaxConcurrent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input int
		want  int
	}{
		{0, DefaultMaxConcurrentTargets},
		{-1, DefaultMaxConcurrentTargets},
		{1, 1},
		{DefaultMaxConcurrentTargets, DefaultMaxConcurrentTargets},
		{30, 30},
	}
	for _, tt := range tests {
		if got := normalizeMaxConcurrent(tt.input); got != tt.want {
			t.Errorf("normalizeMaxConcurrent(%d) = %d, want %d", tt.input, got, tt.want)
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

func TestCheckerCachesSuccessfulDomainLookup(t *testing.T) {
	t.Parallel()

	now := time.Unix(1_700_000_000, 0)
	lookup := &countingExpirationLookup{expiration: now.Add(90 * 24 * time.Hour)}
	checker := NewChecker([]string{"example.com"}, DefaultTimeout, 1)
	checker.Lookup = lookup

	first := checker.Snapshot(context.Background(), now)
	second := checker.Snapshot(context.Background(), now.Add(time.Minute))
	checker.cache.Delete("example.com")
	third := checker.Snapshot(context.Background(), now.Add(2*time.Minute))
	if !first.Success || !second.Success || !third.Success {
		t.Fatalf("cached snapshots success = %v, %v, %v, want all true", first.Success, second.Success, third.Success)
	}
	if calls := lookup.calls.Load(); calls != 2 {
		t.Fatalf("lookup calls = %d, want 2", calls)
	}
	if !second.Domains[0].LookupTime.Equal(now) {
		t.Fatalf("cached lookup time = %v, want %v", second.Domains[0].LookupTime, now)
	}
	if !second.Domains[0].LastKnownGood.Available || !second.Domains[0].LastKnownGood.LastSuccess.Equal(now) {
		t.Fatalf("cached last known good = %#v, want available data from first lookup", second.Domains[0].LastKnownGood)
	}
	stats := checker.CacheStats()
	if stats.Entries != 1 || stats.Hits != 1 || stats.Misses != 2 || stats.Sets != 2 || stats.Deletes != 1 {
		t.Fatalf("cache stats = %#v, want one entry, hit, and delete plus two misses and sets", stats)
	}
}

func TestCheckerPreservesLastKnownGoodAfterTransientFailure(t *testing.T) {
	t.Parallel()

	now := time.Unix(1_700_000_000, 0)
	expiration := now.Add(90 * 24 * time.Hour)
	checker := NewChecker([]string{"example.com"}, DefaultTimeout, 1)
	checker.Lookup = &countingExpirationLookup{expiration: expiration}

	first := checker.Snapshot(context.Background(), now)
	checker.cache.Delete("example.com")
	checker.Lookup = &countingExpirationLookup{err: errors.New("lookup unavailable")}
	second := checker.Snapshot(context.Background(), now.Add(time.Hour))

	if !first.Success || second.Success {
		t.Fatalf("snapshot success = %v, %v, want true then false", first.Success, second.Success)
	}
	got := second.Domains[0].LastKnownGood
	if !got.Available || got.Stale || got.ConsecutiveFailures != 1 || !got.Value.Expiration.Equal(expiration) {
		t.Fatalf("last known good = %#v, want available non-stale expiration with one failure", got)
	}
	if !got.LastSuccess.Equal(now) || !got.LastAttempt.Equal(now.Add(time.Hour)) {
		t.Fatalf("last known good timestamps = %v, %v, want %v, %v", got.LastSuccess, got.LastAttempt, now, now.Add(time.Hour))
	}
}

func TestCheckerMarksLastKnownGoodStale(t *testing.T) {
	t.Parallel()

	now := time.Unix(1_700_000_000, 0)
	checker := NewChecker([]string{"example.com"}, DefaultTimeout, 1)
	checker.Lookup = &countingExpirationLookup{expiration: now.Add(90 * 24 * time.Hour)}
	checker.Snapshot(context.Background(), now)
	checker.cache.Delete("example.com")
	checker.Lookup = &countingExpirationLookup{err: errors.New("lookup unavailable")}

	snapshot := checker.Snapshot(context.Background(), now.Add(registrationDataStaleAfter))
	if !snapshot.Domains[0].LastKnownGood.Stale {
		t.Fatalf("last known good = %#v, want stale data", snapshot.Domains[0].LastKnownGood)
	}
}

func TestCheckerClearsLastKnownGoodWhenDomainIsNotRegistered(t *testing.T) {
	t.Parallel()

	now := time.Unix(1_700_000_000, 0)
	checker := NewChecker([]string{"example.com"}, DefaultTimeout, 1)
	checker.Lookup = &countingExpirationLookup{expiration: now.Add(90 * 24 * time.Hour)}
	checker.Snapshot(context.Background(), now)
	checker.cache.Delete("example.com")
	checker.Lookup = fakeExpirationLookup{
		expirations: map[string]time.Time{"example.com": {}},
		verified:    map[string]bool{"example.com": false},
	}

	snapshot := checker.Snapshot(context.Background(), now.Add(time.Hour))
	if snapshot.Success || snapshot.Domains[0].LastKnownGood.Available {
		t.Fatalf("snapshot = %#v, want failed lookup without last known good data", snapshot)
	}
}

func TestCheckerDoesNotCacheFailedDomainLookup(t *testing.T) {
	t.Parallel()

	now := time.Unix(1_700_000_000, 0)
	lookup := &countingExpirationLookup{err: errors.New("lookup unavailable")}
	checker := NewChecker([]string{"example.com"}, DefaultTimeout, 1)
	checker.Lookup = lookup

	checker.Snapshot(context.Background(), now)
	checker.Snapshot(context.Background(), now.Add(time.Minute))
	if calls := lookup.calls.Load(); calls != 2 {
		t.Fatalf("lookup calls = %d, want failed lookup retried", calls)
	}
	stats := checker.CacheStats()
	if stats.Entries != 0 || stats.Misses != 2 || stats.Sets != 0 {
		t.Fatalf("cache stats = %#v, want two misses and no entries or sets", stats)
	}
}

func TestLookupCacheTTL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		remaining time.Duration
		want      time.Duration
	}{
		{remaining: 31 * 24 * time.Hour, want: 24 * time.Hour},
		{remaining: 30 * 24 * time.Hour, want: 6 * time.Hour},
		{remaining: 8 * 24 * time.Hour, want: 6 * time.Hour},
		{remaining: 7 * 24 * time.Hour, want: time.Hour},
		{remaining: -time.Hour, want: time.Hour},
	}
	for _, tt := range tests {
		if got := lookupCacheTTL(tt.remaining); got != tt.want {
			t.Errorf("lookupCacheTTL(%v) = %v, want %v", tt.remaining, got, tt.want)
		}
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

func (l fakeConcurrentLookup) LookupExpiration(_ context.Context, _ string) (time.Time, bool, error) {
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

	return l.expiration, true, nil
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

func TestCheckerMarksUnverifiedDomainsAsFullCollectionFailure(t *testing.T) {
	t.Parallel()

	now := time.Unix(1_700_000_000, 0)
	snapshot := Checker{
		Targets: []string{"missing.example"},
		Lookup: fakeExpirationLookup{
			verified: map[string]bool{"missing.example": false},
		},
	}.Snapshot(context.Background(), now)

	if snapshot.Success {
		t.Fatal("Snapshot().Success = true, want false for unverified domain")
	}
	if snapshot.Err == nil {
		t.Fatal("Snapshot().Err = nil, want unverified domain error")
	}
	if len(snapshot.Domains) != 1 {
		t.Fatalf("Snapshot().Domains length = %d, want 1", len(snapshot.Domains))
	}
	if !snapshot.Domains[0].Success {
		t.Fatal("domain lookup success = false, want true because registration lookup completed")
	}
	if snapshot.Domains[0].Verified {
		t.Fatal("domain verified = true, want false")
	}
}

func TestCheckerMarksMissingExpirationAsFullCollectionFailure(t *testing.T) {
	t.Parallel()

	now := time.Unix(1_700_000_000, 0)
	snapshot := Checker{
		Targets: []string{"example.com"},
		Lookup: fakeExpirationLookup{
			verified: map[string]bool{"example.com": true},
		},
	}.Snapshot(context.Background(), now)

	if snapshot.Success {
		t.Fatal("Snapshot().Success = true, want false for missing expiration")
	}
	if snapshot.Err == nil {
		t.Fatal("Snapshot().Err = nil, want missing expiration error")
	}
	if len(snapshot.Domains) != 1 {
		t.Fatalf("Snapshot().Domains length = %d, want 1", len(snapshot.Domains))
	}
	if !snapshot.Domains[0].Success {
		t.Fatal("domain lookup success = false, want true because registration lookup completed")
	}
	if !snapshot.Domains[0].Verified {
		t.Fatal("domain verified = false, want true")
	}
	if !snapshot.Domains[0].Expiration.IsZero() {
		t.Fatalf("domain expiration = %v, want zero", snapshot.Domains[0].Expiration)
	}
}
