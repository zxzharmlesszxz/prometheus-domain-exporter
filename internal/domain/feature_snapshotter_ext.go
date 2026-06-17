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
	config, _, _, err := ResolveFeatureConfig(ctx.FeatureName, ctx.Config)
	if err != nil {
		return nil, err
	}
	return newSnapshotEngine(config)
}

func FeatureSnapshotStatus(snapshot Snapshot) framework.SnapshotStatus {
	return framework.SnapshotStatus{
		AttemptTime: snapshot.domain.AttemptTime,
		Success:     snapshot.domain.Success,
	}
}

func newSnapshotEngine(config Config) (featurekit.SnapshotEngine[Snapshot], error) {
	checker := domaincheck.NewChecker(config.Targets, config.Timeout, config.MaxConcurrentTargets)
	var readErrors atomic.Uint64
	var parseErrors atomic.Uint64

	return featurekit.SnapshotEngineFunc[Snapshot](func(ctx context.Context, now time.Time) Snapshot {
		start := time.Now()
		domainSnapshot := checker.Snapshot(ctx, now)
		readErrorCount, parseErrorCount := classifyRDAPSourceErrors(domainSnapshot)
		if readErrorCount > 0 {
			readErrors.Add(readErrorCount)
		}
		if parseErrorCount > 0 {
			parseErrors.Add(parseErrorCount)
		}

		return Snapshot{
			domain: domainSnapshot,
			RDAPResult: framework.FileScrapeResult{
				Path:                  "rdap",
				Up:                    readErrorCount == 0,
				MTimeSeconds:          float64(now.Unix()),
				ReadErrorsTotal:       readErrors.Load(),
				ParseErrorsTotal:      parseErrors.Load(),
				ScrapeDurationSeconds: time.Since(start).Seconds(),
			},
		}
	}), nil
}

func classifyRDAPSourceErrors(snapshot domaincheck.Snapshot) (uint64, uint64) {
	var readErrors uint64
	var parseErrors uint64
	for _, result := range snapshot.Domains {
		if result.Err != nil {
			readErrors++
			continue
		}
		if !result.Verified || result.Expiration.IsZero() {
			parseErrors++
		}
	}
	if snapshot.Err != nil && len(snapshot.Domains) == 0 {
		readErrors++
	}
	return readErrors, parseErrors
}
