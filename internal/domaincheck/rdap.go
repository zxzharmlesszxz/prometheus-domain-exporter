package domaincheck

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	defaultRDAPBootstrapURL = "https://data.iana.org/rdap/dns.json"
	defaultRDAPBootstrapTTL = 24 * time.Hour
)

var errExpirationNotFound = errors.New("expiration event not found")

type RDAPExpirationLookup struct {
	client       *http.Client
	bootstrapURL string
	bootstrapTTL time.Duration

	mu          sync.RWMutex
	services    map[string]string
	fetchedAt   time.Time
	bootstrapMu sync.Mutex
}

func NewRDAPExpirationLookup(timeout time.Duration) *RDAPExpirationLookup {
	timeout = normalizeLookupTimeout(timeout)
	transport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   8,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: timeout,
		ExpectContinueTimeout: 1 * time.Second,
		DisableKeepAlives:     false,
	}
	return newRDAPExpirationLookup(
		&http.Client{
			Timeout:   timeout,
			Transport: transport,
		},
		defaultRDAPBootstrapURL,
	)
}

func newRDAPExpirationLookup(client *http.Client, bootstrapURL string) *RDAPExpirationLookup {
	if client == nil {
		client = http.DefaultClient
	}
	if bootstrapURL == "" {
		bootstrapURL = defaultRDAPBootstrapURL
	}
	return &RDAPExpirationLookup{
		client:       client,
		bootstrapURL: bootstrapURL,
		bootstrapTTL: defaultRDAPBootstrapTTL,
	}
}

func (l *RDAPExpirationLookup) LookupExpiration(ctx context.Context, name string) (_ time.Time, _ bool, err error) {
	name, err = Normalize(name)
	if err != nil {
		return time.Time{}, false, err
	}

	baseURL, err := l.serviceURL(ctx, tld(name))
	if err != nil {
		return time.Time{}, false, err
	}

	endpoint := strings.TrimRight(baseURL, "/") + "/domain/" + url.PathEscape(name)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return time.Time{}, false, fmt.Errorf("create RDAP request: %w", err)
	}
	req.Header.Set("Accept", "application/rdap+json, application/json")

	resp, err := l.client.Do(req)
	if err != nil {
		return time.Time{}, false, err
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("close RDAP response body: %w", closeErr)
		}
	}()

	if resp.StatusCode == http.StatusNotFound {
		return time.Time{}, false, nil
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 4096))
		if readErr != nil {
			return time.Time{}, false, fmt.Errorf("read RDAP error response body: %w", readErr)
		}
		message := strings.TrimSpace(string(body))
		if message == "" {
			message = resp.Status
		}
		return time.Time{}, false, fmt.Errorf("RDAP request %s returned %s: %s", req.URL.String(), resp.Status, message)
	}
	if resp.ContentLength > 1<<20 {
		return time.Time{}, false, fmt.Errorf("RDAP response body too large: %d bytes (max 1MB)", resp.ContentLength)
	}

	var response rdapDomainResponse
	limited := io.LimitReader(resp.Body, 1<<20)
	if err := json.NewDecoder(limited).Decode(&response); err != nil {
		return time.Time{}, false, fmt.Errorf("decode RDAP response: %w", err)
	}

	var peek [1]byte
	if _, err := resp.Body.Read(peek[:]); err == nil {
		return time.Time{}, false, fmt.Errorf("RDAP response body exceeds 1MB limit")
	}

	expiration, err := response.expiration()
	if err != nil {
		if !errors.Is(err, errExpirationNotFound) {
			return time.Time{}, true, err
		}
		return time.Time{}, true, nil
	}
	return expiration, true, nil
}

func (l *RDAPExpirationLookup) serviceURL(ctx context.Context, tld string) (string, error) {
	if service, ok, err := l.cachedServiceURL(tld, time.Now()); ok || err != nil {
		return service, err
	}

	l.bootstrapMu.Lock()
	defer l.bootstrapMu.Unlock()

	if service, ok, err := l.cachedServiceURL(tld, time.Now()); ok || err != nil {
		return service, err
	}

	services, err := l.fetchBootstrap(ctx)
	if err != nil {
		if service, ok := l.staleServiceURL(tld); ok {
			return service, nil
		}
		return "", err
	}

	l.mu.Lock()
	l.services = services
	l.fetchedAt = time.Now()
	service := l.services[tld]
	l.mu.Unlock()

	if service == "" {
		return "", fmt.Errorf("no RDAP service for .%s", tld)
	}

	return service, nil
}

func (l *RDAPExpirationLookup) staleServiceURL(tld string) (string, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if l.services == nil {
		return "", false
	}

	service := l.services[tld]
	return service, service != ""
}

func (l *RDAPExpirationLookup) cachedServiceURL(tld string, now time.Time) (string, bool, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if l.services == nil || !l.bootstrapFresh(now) {
		return "", false, nil
	}

	service := l.services[tld]
	if service == "" {
		return "", true, fmt.Errorf("no RDAP service for .%s", tld)
	}

	return service, true, nil
}

func (l *RDAPExpirationLookup) bootstrapFresh(now time.Time) bool {
	if l.services == nil || l.fetchedAt.IsZero() {
		return false
	}
	ttl := l.bootstrapTTL
	if ttl <= 0 {
		ttl = defaultRDAPBootstrapTTL
	}
	return now.Sub(l.fetchedAt) < ttl
}

func (l *RDAPExpirationLookup) fetchBootstrap(ctx context.Context) (map[string]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, l.bootstrapURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create RDAP bootstrap request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	var bootstrap rdapBootstrap
	if err := l.fetchJSON(req, &bootstrap); err != nil {
		return nil, fmt.Errorf("fetch RDAP bootstrap: %w", err)
	}

	services := make(map[string]string)
	for _, service := range bootstrap.Services {
		if len(service.URLs) == 0 || service.URLs[0] == "" {
			continue
		}
		for _, tld := range service.TLDs {
			services[strings.ToLower(tld)] = service.URLs[0]
		}
	}
	if len(services) == 0 {
		return nil, fmt.Errorf("RDAP bootstrap contains no DNS services")
	}
	return services, nil
}

func (l *RDAPExpirationLookup) fetchJSON(req *http.Request, target any) (err error) {
	resp, err := l.client.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("close RDAP response body: %w", closeErr)
		}
	}()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 4096))
		if readErr != nil {
			return fmt.Errorf("read RDAP error response body: %w", readErr)
		}
		message := strings.TrimSpace(string(body))
		if message == "" {
			message = resp.Status
		}
		return fmt.Errorf("RDAP request %s returned %s: %s", req.URL.String(), resp.Status, message)
	}
	if resp.ContentLength > 1<<20 {
		return fmt.Errorf("RDAP response body too large: %d bytes (max 1MB)", resp.ContentLength)
	}

	limited := io.LimitReader(resp.Body, 1<<20)
	if err := json.NewDecoder(limited).Decode(target); err != nil {
		return fmt.Errorf("decode RDAP response: %w", err)
	}

	var peek [1]byte
	if _, err := resp.Body.Read(peek[:]); err == nil {
		return fmt.Errorf("RDAP response body exceeds 1MB limit")
	}
	return nil
}

type rdapBootstrap struct {
	Services []rdapService `json:"services"`
}

type rdapService struct {
	TLDs []string
	URLs []string
}

func (s *rdapService) UnmarshalJSON(data []byte) error {
	var parts []json.RawMessage
	if err := json.Unmarshal(data, &parts); err != nil {
		return err
	}
	if len(parts) != 2 {
		return fmt.Errorf("RDAP bootstrap service has %d parts, want 2", len(parts))
	}
	if err := json.Unmarshal(parts[0], &s.TLDs); err != nil {
		return fmt.Errorf("decode RDAP bootstrap TLDs: %w", err)
	}
	if err := json.Unmarshal(parts[1], &s.URLs); err != nil {
		return fmt.Errorf("decode RDAP bootstrap URLs: %w", err)
	}
	return nil
}

type rdapDomainResponse struct {
	Events []rdapEvent `json:"events"`
}

type rdapEvent struct {
	Action string `json:"eventAction"`
	Date   string `json:"eventDate"`
}

func (r rdapDomainResponse) expiration() (time.Time, error) {
	var fallback time.Time
	for _, event := range r.Events {
		action := strings.ToLower(strings.TrimSpace(event.Action))
		if action != "expiration" && action != "registration expiration" && !strings.Contains(action, "expir") {
			continue
		}
		parsed, err := time.Parse(time.RFC3339Nano, event.Date)
		if err != nil {
			if action == "expiration" || action == "registration expiration" {
				return time.Time{}, fmt.Errorf("parse expiration date %q: %w", event.Date, err)
			}
			continue
		}
		if action == "expiration" || action == "registration expiration" {
			return parsed, nil
		}
		if fallback.IsZero() {
			fallback = parsed
		}
	}
	if !fallback.IsZero() {
		return fallback, nil
	}
	return time.Time{}, errExpirationNotFound
}
