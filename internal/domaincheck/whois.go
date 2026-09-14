package domaincheck

import (
	"context"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"
)

const (
	defaultWHOISBootstrapServer       = "whois.iana.org"
	defaultWHOISBootstrapTTL          = 24 * time.Hour
	maxWHOISResponseBodyBytes   int64 = 1 << 20
)

type whoisQueryFunc func(context.Context, string, string) (string, error)
type whoisAttemptFunc func(context.Context, time.Duration, string, string) (string, error)

type whoisService struct {
	server    string
	fetchedAt time.Time
}

type WHOISExpirationLookup struct {
	query           whoisQueryFunc
	bootstrapServer string
	bootstrapTTL    time.Duration

	mu          sync.RWMutex
	services    map[string]whoisService
	bootstrapMu sync.Mutex
}

func NewWHOISExpirationLookup(timeout time.Duration) *WHOISExpirationLookup {
	timeout = normalizeLookupTimeout(timeout)
	return newWHOISExpirationLookup(
		func(ctx context.Context, server, query string) (string, error) {
			return queryWHOIS(ctx, timeout, server, query)
		},
		defaultWHOISBootstrapServer,
	)
}

func newWHOISExpirationLookup(query whoisQueryFunc, bootstrapServer string) *WHOISExpirationLookup {
	if query == nil {
		timeout := DefaultTimeout
		query = func(ctx context.Context, server, value string) (string, error) {
			return queryWHOIS(ctx, timeout, server, value)
		}
	}
	if bootstrapServer == "" {
		bootstrapServer = defaultWHOISBootstrapServer
	}
	return &WHOISExpirationLookup{
		query:           query,
		bootstrapServer: bootstrapServer,
		bootstrapTTL:    defaultWHOISBootstrapTTL,
		services:        make(map[string]whoisService),
	}
}

func (l *WHOISExpirationLookup) LookupExpiration(ctx context.Context, name string) (time.Time, bool, error) {
	name, err := Normalize(name)
	if err != nil {
		return time.Time{}, false, err
	}

	server, err := l.serviceServer(ctx, tld(name))
	if err != nil {
		return time.Time{}, false, err
	}
	response, err := l.query(ctx, server, name)
	if err != nil {
		return time.Time{}, false, fmt.Errorf("query WHOIS server %s: %w", server, err)
	}
	return parseWHOISExpiration(response)
}

func (l *WHOISExpirationLookup) serviceServer(ctx context.Context, tld string) (string, error) {
	if server, ok := l.cachedServiceServer(tld, time.Now()); ok {
		return server, nil
	}

	l.bootstrapMu.Lock()
	defer l.bootstrapMu.Unlock()

	if server, ok := l.cachedServiceServer(tld, time.Now()); ok {
		return server, nil
	}

	response, err := l.query(ctx, l.bootstrapServer, tld)
	if err != nil {
		return "", fmt.Errorf("discover WHOIS service for .%s: %w", tld, err)
	}
	server := whoisField(response, "whois")
	if server == "" {
		return "", fmt.Errorf("no published WHOIS service for .%s", tld)
	}

	l.mu.Lock()
	l.services[tld] = whoisService{server: server, fetchedAt: time.Now()}
	l.mu.Unlock()
	return server, nil
}

func (l *WHOISExpirationLookup) cachedServiceServer(tld string, now time.Time) (string, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	service, ok := l.services[tld]
	if !ok || service.server == "" || service.fetchedAt.IsZero() {
		return "", false
	}
	ttl := l.bootstrapTTL
	if ttl <= 0 {
		ttl = defaultWHOISBootstrapTTL
	}
	if now.Sub(service.fetchedAt) >= ttl {
		return "", false
	}
	return service.server, true
}

func queryWHOIS(ctx context.Context, timeout time.Duration, server, query string) (string, error) {
	return queryWHOISWithRetry(ctx, timeout, server, query, queryWHOISOnce)
}

func queryWHOISWithRetry(ctx context.Context, timeout time.Duration, server, query string, attempt whoisAttemptFunc) (string, error) {
	attemptTimeout := timeout / 2
	if attemptTimeout <= 0 {
		attemptTimeout = timeout
	}

	var lastErr error
	for range 2 {
		attemptCtx, cancel := context.WithTimeout(ctx, attemptTimeout)
		response, err := attempt(attemptCtx, attemptTimeout, server, query)
		cancel()
		if err == nil {
			return response, nil
		}
		lastErr = err
		if ctx.Err() != nil {
			break
		}
	}
	return "", lastErr
}

func queryWHOISOnce(ctx context.Context, timeout time.Duration, server, query string) (_ string, err error) {
	address := whoisAddress(server)
	dialer := net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return "", err
	}
	defer func() {
		if closeErr := conn.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("close WHOIS connection: %w", closeErr)
		}
	}()

	deadline := time.Now().Add(timeout)
	if contextDeadline, ok := ctx.Deadline(); ok && contextDeadline.Before(deadline) {
		deadline = contextDeadline
	}
	if err := conn.SetDeadline(deadline); err != nil {
		return "", fmt.Errorf("set WHOIS connection deadline: %w", err)
	}
	if _, err := io.WriteString(conn, query+"\r\n"); err != nil {
		return "", fmt.Errorf("write WHOIS query: %w", err)
	}

	data, err := io.ReadAll(io.LimitReader(conn, maxWHOISResponseBodyBytes+1))
	if err != nil {
		return "", fmt.Errorf("read WHOIS response: %w", err)
	}
	if int64(len(data)) > maxWHOISResponseBodyBytes {
		return "", fmt.Errorf("WHOIS response body exceeds 1MB limit")
	}
	return string(data), nil
}

func whoisAddress(server string) string {
	server = strings.TrimSpace(server)
	server = strings.TrimPrefix(server, "whois://")
	server = strings.TrimRight(server, "/")
	if _, _, err := net.SplitHostPort(server); err == nil {
		return server
	}
	return net.JoinHostPort(server, "43")
}

func parseWHOISExpiration(response string) (time.Time, bool, error) {
	if whoisNotFound(response) {
		return time.Time{}, false, nil
	}

	value := whoisField(response,
		"Registry Expiry Date",
		"Registrar Registration Expiration Date",
		"Expiration Date",
		"Expiry Date",
		"Expiration Time",
		"Expires On",
		"expires",
		"expire",
		"expire-date",
		"paid-till",
	)
	if value == "" {
		return time.Time{}, true, nil
	}

	for _, layout := range []string{
		time.RFC3339Nano,
		"2006-01-02 15:04:05Z07:00",
		"2006-01-02 15:04:05",
		"2006-01-02",
		"02-Jan-2006",
		"2006.01.02",
		"2006/01/02",
		"02/01/2006",
		"20060102",
	} {
		if expiration, err := time.Parse(layout, value); err == nil {
			return expiration, true, nil
		}
	}
	return time.Time{}, true, newLookupParseError(fmt.Errorf("parse WHOIS expiration date %q", value))
}

func whoisField(response string, names ...string) string {
	lines := strings.Split(response, "\n")
	for _, name := range names {
		for _, line := range lines {
			key, value, ok := strings.Cut(line, ":")
			if !ok {
				continue
			}
			if strings.EqualFold(strings.TrimSpace(key), name) {
				return strings.TrimSpace(value)
			}
		}
	}
	return ""
}

func whoisNotFound(response string) bool {
	for line := range strings.SplitSeq(response, "\n") {
		line = strings.ToLower(strings.Trim(strings.TrimSpace(line), "%# "))
		if strings.HasPrefix(line, "no match") ||
			strings.HasPrefix(line, "not found") ||
			strings.HasPrefix(line, "domain not found") ||
			strings.HasPrefix(line, "no data found") ||
			strings.HasPrefix(line, "no entries found") ||
			strings.HasPrefix(line, "no object found") {
			return true
		}
	}
	return false
}
