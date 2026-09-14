package domaincheck

import (
	"context"
	"errors"
	"time"

	"github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter/featurekit"
)

const (
	SourceRDAP  = "rdap"
	SourceWHOIS = "whois"
)

type Result struct {
	Name          string
	LookupTime    time.Time
	Expiration    time.Time
	Source        string
	LastKnownGood featurekit.LastKnownGoodResult[RegistrationData]
	// Success means the registration lookup completed without a transport, bootstrap, or service error.
	Success bool
	// Verified means the registration source confirmed the domain is registered.
	Verified bool
	Err      error
}

type RegistrationData struct {
	Expiration time.Time
}

type ExpirationLookup interface {
	LookupExpiration(context.Context, string) (expiration time.Time, verified bool, err error)
}

type SourcedExpirationLookup interface {
	LookupExpirationSource(context.Context, string) (expiration time.Time, verified bool, source string, err error)
}

type lookupParseError struct {
	err error
}

func (e lookupParseError) Error() string {
	return e.err.Error()
}

func (e lookupParseError) Unwrap() error {
	return e.err
}

func newLookupParseError(err error) error {
	return lookupParseError{err: err}
}

func IsLookupParseError(err error) bool {
	var parseErr lookupParseError
	return errors.As(err, &parseErr)
}

type Snapshot struct {
	AttemptTime time.Time
	Success     bool
	Domains     []Result
	Err         error
}
