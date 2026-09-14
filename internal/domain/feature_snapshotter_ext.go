package domain

import (
	"context"
	"sync/atomic"
	"time"

	framework "github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter"
	"github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter/featurekit"

	"github.com/zxzharmlesszxz/prometheus-domain-exporter/internal/domaincheck"
)

func NewDefaultSnapshotEngine() featurekit.SnapshotEngine[Snapshot] {
	engine, err := newSnapshotEngine(NewDefaultConfig())
	if err != nil {
		panic(err)
	}
	return engine
}

func NewSnapshotEngine(ctx featurekit.CollectorContext[Config]) (featurekit.SnapshotEngine[Snapshot], error) {
	return newSnapshotEngine(ctx.Config)
}

func FeatureSnapshotStatus(snapshot Snapshot) framework.SnapshotStatus {
	return framework.SnapshotStatus{
		AttemptTime: snapshot.domain.AttemptTime,
		Success:     snapshot.domain.Success,
	}
}

func newSnapshotEngine(config Config) (featurekit.SnapshotEngine[Snapshot], error) {
	checker := domaincheck.NewChecker(config.Targets, config.Timeout, config.MaxConcurrentTargets)
	rdapCounters := newSourceErrorCounters()
	whoisCounters := newSourceErrorCounters()

	return featurekit.SnapshotEngineFunc[Snapshot](func(ctx context.Context, now time.Time) Snapshot {
		start := time.Now()
		domainSnapshot := checker.Snapshot(ctx, now)
		duration := time.Since(start).Seconds()
		rdapResult, rdapValid := buildSourceResult(domainSnapshot, domaincheck.SourceRDAP, now, duration, rdapCounters)
		whoisResult, whoisValid := buildSourceResult(domainSnapshot, domaincheck.SourceWHOIS, now, duration, whoisCounters)

		return Snapshot{
			domain:      domainSnapshot,
			CacheStats:  checker.CacheStats(),
			RDAPResult:  rdapResult,
			RDAPValid:   rdapValid,
			WHOISResult: whoisResult,
			WHOISValid:  whoisValid,
		}
	}), nil
}

type sourceErrorCounters struct {
	read  atomic.Uint64
	parse atomic.Uint64
}

func newSourceErrorCounters() *sourceErrorCounters {
	return &sourceErrorCounters{}
}

func buildSourceResult(snapshot domaincheck.Snapshot, source string, now time.Time, duration float64, counters *sourceErrorCounters) (framework.FileScrapeResult, bool) {
	used, readErrorCount, parseErrorCount := classifySourceErrors(snapshot, source)
	if !used {
		return framework.FileScrapeResult{}, false
	}
	counters.read.Add(readErrorCount)
	counters.parse.Add(parseErrorCount)
	mtime := latestSourceLookupTime(snapshot, source)
	if mtime.IsZero() {
		mtime = now
	}
	return framework.FileScrapeResult{
		Path:                  source,
		Up:                    readErrorCount == 0,
		MTimeSeconds:          float64(mtime.Unix()),
		ReadErrorsTotal:       counters.read.Load(),
		ParseErrorsTotal:      counters.parse.Load(),
		ScrapeDurationSeconds: duration,
	}, readErrorCount == 0 && parseErrorCount == 0
}

func latestSourceLookupTime(snapshot domaincheck.Snapshot, source string) time.Time {
	var latest time.Time
	for _, result := range snapshot.Domains {
		if result.Source == source && result.LookupTime.After(latest) {
			latest = result.LookupTime
		}
	}
	return latest
}

func classifySourceErrors(snapshot domaincheck.Snapshot, source string) (bool, uint64, uint64) {
	used := len(snapshot.Domains) == 0 && source == domaincheck.SourceRDAP
	var readErrors uint64
	var parseErrors uint64
	for _, result := range snapshot.Domains {
		if result.Source != source {
			continue
		}
		used = true
		if result.Err != nil {
			if domaincheck.IsLookupParseError(result.Err) {
				parseErrors++
			} else {
				readErrors++
			}
			continue
		}
		if !result.Verified || result.Expiration.IsZero() {
			parseErrors++
		}
	}
	if snapshot.Err != nil && len(snapshot.Domains) == 0 {
		readErrors++
	}
	return used, readErrors, parseErrors
}
