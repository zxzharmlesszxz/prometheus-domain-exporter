package domaincheck

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter/featurekit"
	"golang.org/x/net/idna"
)

const (
	DefaultTimeout              = 10 * time.Second
	DefaultMaxConcurrentTargets = 8
	distantExpirationThreshold  = 30 * 24 * time.Hour
	nearExpirationThreshold     = 7 * 24 * time.Hour
	distantLookupCacheTTL       = 24 * time.Hour
	nearLookupCacheTTL          = 6 * time.Hour
	criticalLookupCacheTTL      = time.Hour
)

type Checker struct {
	Targets              []string
	Lookup               ExpirationLookup
	Timeout              time.Duration
	MaxConcurrentTargets int
	cache                *featurekit.TTLCache[string, Result]
}

func NewChecker(domains []string, lookupTimeout time.Duration, maxConcurrent int) Checker {
	lookupTimeout = normalizeLookupTimeout(lookupTimeout)
	maxConcurrent = normalizeMaxConcurrent(maxConcurrent)
	return Checker{
		Targets:              domains,
		Lookup:               NewRegistrationExpirationLookup(lookupTimeout),
		Timeout:              lookupTimeout,
		MaxConcurrentTargets: maxConcurrent,
		cache:                featurekit.NewTTLCache[string, Result](0),
	}
}

func (c Checker) CacheStats() featurekit.TTLCacheStats {
	return c.cache.Stats()
}

type domainCheckJob struct {
	Index int
	Name  string
}

func (c Checker) Snapshot(ctx context.Context, now time.Time) Snapshot {
	snapshot := Snapshot{
		AttemptTime: now,
		Success:     true,
	}
	if len(c.Targets) == 0 {
		return snapshot
	}

	lookupTimeout := normalizeLookupTimeout(c.Timeout)
	lookup := c.Lookup
	if lookup == nil {
		lookup = NewRegistrationExpirationLookup(lookupTimeout)
	}
	cache := c.cache
	if cache == nil {
		cache = featurekit.NewTTLCache[string, Result](0)
	}

	results := make([]Result, len(c.Targets))
	jobs := make(chan domainCheckJob)
	var wg sync.WaitGroup
	sentJobs := 0
	totalTargets := len(c.Targets)

	maxConcurrent := c.MaxConcurrentTargets
	maxConcurrent = min(normalizeMaxConcurrent(maxConcurrent), len(c.Targets))

	for i := 0; i < maxConcurrent; i++ {
		wg.Go(func() {
			for job := range jobs {
				if cached, ok := cache.Get(job.Name); ok {
					results[job.Index] = cached
					continue
				}

				domainCtx, cancel := context.WithTimeout(ctx, lookupTimeout)
				expiration, verified, source, err := lookupExpiration(domainCtx, lookup, job.Name)
				cancel()

				name := job.Name
				if unicodeName, err := idna.ToUnicode(job.Name); err == nil {
					name = unicodeName
				}

				result := Result{
					Name:       name,
					LookupTime: now,
					Expiration: expiration,
					Source:     source,
					Success:    err == nil,
					Verified:   verified,
					Err:        err,
				}
				results[job.Index] = result
				if fullCollectionTargetError(result) == nil {
					cache.SetWithTTL(job.Name, result, lookupCacheTTL(result.Expiration.Sub(now)))
				}
			}
		})
	}

sendJobs:
	for i, name := range c.Targets {
		select {
		case <-ctx.Done():
			break sendJobs
		case jobs <- domainCheckJob{Index: i, Name: name}:
			sentJobs++
		}
	}
	close(jobs)
	wg.Wait()

	var firstLookupErr error
	for i, result := range results {
		if i >= sentJobs {
			break
		}
		if targetErr := fullCollectionTargetError(result); targetErr != nil {
			snapshot.Success = false
			if firstLookupErr == nil {
				firstLookupErr = fmt.Errorf("lookup %s registration expiration: %w", result.Name, targetErr)
			}
		}
		snapshot.Domains = append(snapshot.Domains, result)
	}
	if ctx.Err() != nil {
		snapshot.Success = false
		if firstLookupErr != nil {
			snapshot.Err = fmt.Errorf("context canceled after %d/%d targets: %w", sentJobs, totalTargets, firstLookupErr)
		} else {
			snapshot.Err = ctx.Err()
		}
	} else {
		snapshot.Err = firstLookupErr
	}
	return snapshot
}

func lookupCacheTTL(expirationRemaining time.Duration) time.Duration {
	switch {
	case expirationRemaining > distantExpirationThreshold:
		return distantLookupCacheTTL
	case expirationRemaining > nearExpirationThreshold:
		return nearLookupCacheTTL
	default:
		return criticalLookupCacheTTL
	}
}

func lookupExpiration(ctx context.Context, lookup ExpirationLookup, name string) (time.Time, bool, string, error) {
	if sourced, ok := lookup.(SourcedExpirationLookup); ok {
		return sourced.LookupExpirationSource(ctx, name)
	}
	expiration, verified, err := lookup.LookupExpiration(ctx, name)
	return expiration, verified, SourceRDAP, err
}

func fullCollectionTargetError(result Result) error {
	if result.Err != nil {
		return result.Err
	}
	if !result.Verified {
		return fmt.Errorf("domain was not verified by RDAP")
	}
	if result.Expiration.IsZero() {
		return fmt.Errorf("registration expiration was not found")
	}
	return nil
}

func normalizeLookupTimeout(timeout time.Duration) time.Duration {
	if timeout <= 0 {
		return DefaultTimeout
	}
	return timeout
}

func normalizeMaxConcurrent(maxConcurrent int) int {
	if maxConcurrent <= 0 {
		return DefaultMaxConcurrentTargets
	}
	return maxConcurrent
}
