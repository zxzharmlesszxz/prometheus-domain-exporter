package domaincheck

import (
	"context"
	"fmt"
	"time"
)

const DefaultLookupTimeout = 10 * time.Second

type Checker struct {
	Domains       []string
	Lookup        ExpirationLookup
	LookupTimeout time.Duration
}

func NewChecker(domains []string, lookupTimeout time.Duration) Checker {
	lookupTimeout = normalizeLookupTimeout(lookupTimeout)
	return Checker{
		Domains:       domains,
		Lookup:        NewRDAPExpirationLookup(lookupTimeout),
		LookupTimeout: lookupTimeout,
	}
}

func (c Checker) Snapshot(ctx context.Context, now time.Time) Snapshot {
	snapshot := Snapshot{
		AttemptTime: now,
		Success:     true,
	}
	if len(c.Domains) == 0 {
		return snapshot
	}

	lookupTimeout := normalizeLookupTimeout(c.LookupTimeout)
	lookup := c.Lookup
	if lookup == nil {
		lookup = NewRDAPExpirationLookup(lookupTimeout)
	}

	for _, name := range c.Domains {
		domainCtx, cancel := context.WithTimeout(ctx, lookupTimeout)
		expiration, err := lookup.LookupExpiration(domainCtx, name)
		cancel()

		result := Result{
			Name:       name,
			LookupTime: now,
			Expiration: expiration,
			Success:    err == nil,
			Err:        err,
		}
		if err != nil {
			snapshot.Success = false
			if snapshot.Err == nil {
				snapshot.Err = fmt.Errorf("lookup %s registration expiration: %w", name, err)
			}
		}
		snapshot.Domains = append(snapshot.Domains, result)
	}
	return snapshot
}

func normalizeLookupTimeout(timeout time.Duration) time.Duration {
	if timeout <= 0 {
		return DefaultLookupTimeout
	}
	return timeout
}
