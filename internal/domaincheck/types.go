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
	Verified   bool
	Err        error
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
