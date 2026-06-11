package domaincheck

import (
	"context"
	"fmt"
	"sync"
	"time"

	"golang.org/x/net/idna"
)

const (
	DefaultTimeout              = 10 * time.Second
	DefaultMaxConcurrentTargets = 8
)

type Checker struct {
	Targets              []string
	Lookup               ExpirationLookup
	Timeout              time.Duration
	MaxConcurrentTargets int
}

func NewChecker(domains []string, lookupTimeout time.Duration, maxConcurrent int) Checker {
	if maxConcurrent <= 0 {
		maxConcurrent = DefaultMaxConcurrentTargets
	}
	return Checker{
		Targets:              domains,
		Lookup:               NewRDAPExpirationLookup(lookupTimeout),
		Timeout:              lookupTimeout,
		MaxConcurrentTargets: maxConcurrent,
	}
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
		lookup = NewRDAPExpirationLookup(lookupTimeout)
	}

	results := make([]Result, len(c.Targets))
	jobs := make(chan domainCheckJob)
	var wg sync.WaitGroup
	sentJobs := 0
	totalTargets := len(c.Targets)

	maxConcurrent := c.MaxConcurrentTargets
	if maxConcurrent <= 0 {
		maxConcurrent = DefaultMaxConcurrentTargets
	}
	if maxConcurrent > len(c.Targets) {
		maxConcurrent = len(c.Targets)
	}

	for i := 0; i < maxConcurrent; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				domainCtx, cancel := context.WithTimeout(ctx, lookupTimeout)
				expiration, verified, err := lookup.LookupExpiration(domainCtx, job.Name)
				cancel()

				name := job.Name
				if unicodeName, err := idna.ToUnicode(job.Name); err == nil {
					name = unicodeName
				}

				results[job.Index] = Result{
					Name:       name,
					LookupTime: now,
					Expiration: expiration,
					Success:    err == nil,
					Verified:   verified,
					Err:        err,
				}
			}
		}()
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
