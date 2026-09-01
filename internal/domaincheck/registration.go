package domaincheck

import (
	"context"
	"errors"
	"time"
)

type RegistrationExpirationLookup struct {
	RDAP  *RDAPExpirationLookup
	WHOIS *WHOISExpirationLookup
}

func NewRegistrationExpirationLookup(timeout time.Duration) *RegistrationExpirationLookup {
	return &RegistrationExpirationLookup{
		RDAP:  NewRDAPExpirationLookup(timeout),
		WHOIS: NewWHOISExpirationLookup(timeout),
	}
}

func (l *RegistrationExpirationLookup) LookupExpiration(ctx context.Context, name string) (time.Time, bool, error) {
	expiration, verified, _, err := l.LookupExpirationSource(ctx, name)
	return expiration, verified, err
}

func (l *RegistrationExpirationLookup) LookupExpirationSource(ctx context.Context, name string) (time.Time, bool, string, error) {
	expiration, verified, err := l.RDAP.LookupExpiration(ctx, name)
	if err == nil {
		return expiration, verified, SourceRDAP, nil
	}

	if _, ok := errors.AsType[rdapServiceNotFoundError](err); !ok {
		return time.Time{}, false, SourceRDAP, err
	}

	expiration, verified, err = l.WHOIS.LookupExpiration(ctx, name)
	return expiration, verified, SourceWHOIS, err
}
