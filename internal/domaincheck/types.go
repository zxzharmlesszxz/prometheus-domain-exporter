package domaincheck

import (
	"context"
	"time"
)

type Result struct {
	Name       string
	LookupTime time.Time
	Expiration time.Time
	// Success means the RDAP lookup completed without a transport, bootstrap, or service error.
	Success bool
	// Verified means the RDAP response confirmed the domain is registered.
	Verified bool
	Err      error
}

type ExpirationLookup interface {
	LookupExpiration(context.Context, string) (expiration time.Time, verified bool, err error)
}

type Snapshot struct {
	AttemptTime time.Time
	Success     bool
	Domains     []Result
	Err         error
}
