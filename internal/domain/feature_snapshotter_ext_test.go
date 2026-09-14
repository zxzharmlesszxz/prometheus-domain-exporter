package domain

import (
	"errors"
	"testing"
	"time"

	"github.com/zxzharmlesszxz/prometheus-domain-exporter/internal/domaincheck"
)

func TestClassifySourceErrors(t *testing.T) {
	t.Parallel()

	snapshot := domaincheck.Snapshot{
		Domains: []domaincheck.Result{
			{Source: domaincheck.SourceRDAP, Success: true, Verified: true, Expiration: time.Now()},
			{Source: domaincheck.SourceWHOIS, Err: errors.New("whois unavailable")},
			{Source: domaincheck.SourceWHOIS, Success: true, Verified: true},
		},
	}

	used, readErrors, parseErrors := classifySourceErrors(snapshot, domaincheck.SourceRDAP)
	if !used || readErrors != 0 || parseErrors != 0 {
		t.Fatalf("RDAP classification = %v, %d, %d, want true, 0, 0", used, readErrors, parseErrors)
	}
	used, readErrors, parseErrors = classifySourceErrors(snapshot, domaincheck.SourceWHOIS)
	if !used || readErrors != 1 || parseErrors != 1 {
		t.Fatalf("WHOIS classification = %v, %d, %d, want true, 1, 1", used, readErrors, parseErrors)
	}
}

func TestClassifySourceErrorsKeepsEmptyRDAPSourceHealthy(t *testing.T) {
	t.Parallel()

	used, readErrors, parseErrors := classifySourceErrors(domaincheck.Snapshot{}, domaincheck.SourceRDAP)
	if !used || readErrors != 0 || parseErrors != 0 {
		t.Fatalf("RDAP classification = %v, %d, %d, want true, 0, 0", used, readErrors, parseErrors)
	}
	used, _, _ = classifySourceErrors(domaincheck.Snapshot{}, domaincheck.SourceWHOIS)
	if used {
		t.Fatal("WHOIS classification used = true, want false without WHOIS targets")
	}
}

func TestLatestSourceLookupTime(t *testing.T) {
	t.Parallel()

	older := time.Unix(1_700_000_000, 0)
	newer := older.Add(time.Hour)
	snapshot := domaincheck.Snapshot{Domains: []domaincheck.Result{
		{Source: domaincheck.SourceRDAP, LookupTime: older},
		{Source: domaincheck.SourceWHOIS, LookupTime: newer},
		{Source: domaincheck.SourceRDAP, LookupTime: newer},
	}}

	if got := latestSourceLookupTime(snapshot, domaincheck.SourceRDAP); !got.Equal(newer) {
		t.Fatalf("latest RDAP lookup time = %v, want %v", got, newer)
	}
	if got := latestSourceLookupTime(snapshot, domaincheck.SourceWHOIS); !got.Equal(newer) {
		t.Fatalf("latest WHOIS lookup time = %v, want %v", got, newer)
	}
}
