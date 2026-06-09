package domaincheck

import (
	"context"
	"time"
)

type Result struct {
	Name       string
	LookupTime time.Time
	Expiration time.Time
	Success    bool
	Err        error
}

type ExpirationLookup interface {
	LookupExpiration(context.Context, string) (time.Time, error)
}

type Snapshot struct {
	AttemptTime time.Time
	Success     bool
	Domains     []Result
	Err         error
}
