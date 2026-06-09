package domaincheck

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRDAPExpirationLookup(t *testing.T) {
	t.Parallel()

	expiration := "2030-01-02T03:04:05Z"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/dns.json":
			writeResponsef(t, w, `{"services":[[["example"],["%s/rdap/"]]]}`, serverURL(t, r))
		case "/rdap/domain/example.example":
			writeResponsef(t, w, `{"events":[{"eventAction":"registration expiration","eventDate":%q}]}`, expiration)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	lookup := newRDAPExpirationLookup(server.Client(), server.URL+"/dns.json")
	got, err := lookup.LookupExpiration(context.Background(), "Example.EXAMPLE.")
	if err != nil {
		t.Fatalf("LookupExpiration() error = %v, want nil", err)
	}
	want, err := time.Parse(time.RFC3339, expiration)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if !got.Equal(want) {
		t.Fatalf("LookupExpiration() = %v, want %v", got, want)
	}
}

func TestRDAPExpirationLookupReportsMissingExpiration(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/dns.json":
			writeResponsef(t, w, `{"services":[[["example"],["%s/rdap/"]]]}`, serverURL(t, r))
		case "/rdap/domain/example.example":
			writeResponse(t, w, `{"events":[{"eventAction":"registration","eventDate":"2020-01-02T03:04:05Z"}]}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	lookup := newRDAPExpirationLookup(server.Client(), server.URL+"/dns.json")
	if _, err := lookup.LookupExpiration(context.Background(), "example.example"); err == nil {
		t.Fatal("LookupExpiration() error = nil, want error")
	}
}

func TestRDAPExpirationLookupReportsMissingService(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/dns.json" {
			http.NotFound(w, r)
			return
		}
		writeResponsef(t, w, `{"services":[[["other"],["%s/rdap/"]]]}`, serverURL(t, r))
	}))
	defer server.Close()

	lookup := newRDAPExpirationLookup(server.Client(), server.URL+"/dns.json")
	if _, err := lookup.LookupExpiration(context.Background(), "example.example"); err == nil {
		t.Fatal("LookupExpiration() error = nil, want missing service error")
	}
}

func TestRDAPExpirationLookupReportsHTTPError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/dns.json":
			writeResponsef(t, w, `{"services":[[["example"],["%s/rdap/"]]]}`, serverURL(t, r))
		case "/rdap/domain/example.example":
			http.Error(w, "registry unavailable", http.StatusServiceUnavailable)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	lookup := newRDAPExpirationLookup(server.Client(), server.URL+"/dns.json")
	if _, err := lookup.LookupExpiration(context.Background(), "example.example"); err == nil {
		t.Fatal("LookupExpiration() error = nil, want HTTP error")
	}
}

func TestNewRDAPExpirationLookupSetsTransport(t *testing.T) {
	t.Parallel()

	lookup := NewRDAPExpirationLookup(5 * time.Second)

	if lookup.client.Timeout != 5*time.Second {
		t.Fatalf("client.Timeout = %v, want 5s", lookup.client.Timeout)
	}
	transport, ok := lookup.client.Transport.(*http.Transport)
	if !ok {
		t.Fatal("client.Transport is not *http.Transport")
	}
	if transport.MaxIdleConnsPerHost != 2 {
		t.Fatalf("MaxIdleConnsPerHost = %d, want 2", transport.MaxIdleConnsPerHost)
	}
	if transport.DisableKeepAlives {
		t.Fatal("DisableKeepAlives = true, want false")
	}
}

func TestNewRDAPExpirationLookupUsesDefaults(t *testing.T) {
	t.Parallel()

	lookup := NewRDAPExpirationLookup(0)
	if lookup.client.Timeout != DefaultLookupTimeout {
		t.Fatalf("client timeout = %v, want %v", lookup.client.Timeout, DefaultLookupTimeout)
	}
	if lookup.bootstrapURL != defaultRDAPBootstrapURL {
		t.Fatalf("bootstrapURL = %q, want %q", lookup.bootstrapURL, defaultRDAPBootstrapURL)
	}

	lookup = newRDAPExpirationLookup(nil, "")
	if lookup.client != http.DefaultClient {
		t.Fatal("client = custom, want http.DefaultClient")
	}
	if lookup.bootstrapURL != defaultRDAPBootstrapURL {
		t.Fatalf("bootstrapURL = %q, want %q", lookup.bootstrapURL, defaultRDAPBootstrapURL)
	}
}

func TestRDAPServiceURLUsesCachedServices(t *testing.T) {
	t.Parallel()

	lookup := newRDAPExpirationLookup(nil, "")
	lookup.services = map[string]string{
		"example": "https://rdap.example.test",
	}

	got, err := lookup.serviceURL(context.Background(), "example")
	if err != nil {
		t.Fatalf("serviceURL() error = %v, want nil", err)
	}
	if got != "https://rdap.example.test" {
		t.Fatalf("serviceURL() = %q, want cached service", got)
	}
	if _, err := lookup.serviceURL(context.Background(), "missing"); err == nil {
		t.Fatal("serviceURL(missing) error = nil, want missing service error")
	}
}

func TestRDAPFetchBootstrapRejectsEmptyServices(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeResponse(t, w, `{"services":[[["example"],[]],[["net"],[""]]]}`)
	}))
	defer server.Close()

	lookup := newRDAPExpirationLookup(server.Client(), server.URL)
	if _, err := lookup.fetchBootstrap(context.Background()); err == nil {
		t.Fatal("fetchBootstrap() error = nil, want empty services error")
	}
}

func TestRDAPFetchJSONReportsDecodeError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeResponse(t, w, `{`)
	}))
	defer server.Close()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL, nil)
	if err != nil {
		t.Fatalf("NewRequestWithContext() error = %v", err)
	}
	lookup := newRDAPExpirationLookup(server.Client(), "")
	var response rdapBootstrap
	if err := lookup.fetchJSON(req, &response); err == nil {
		t.Fatal("fetchJSON() error = nil, want decode error")
	}
}

func TestRDAPFetchJSONRejectsContentLengthOverLimit(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", "1048577")
		writeResponse(t, w, `{}`)
	}))
	defer server.Close()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL, nil)
	if err != nil {
		t.Fatalf("NewRequestWithContext() error = %v", err)
	}
	lookup := newRDAPExpirationLookup(server.Client(), "")
	var response rdapBootstrap
	if err := lookup.fetchJSON(req, &response); err == nil {
		t.Fatal("fetchJSON() error = nil, want body too large error")
	}
}

func TestRDAPFetchJSONDetectsTruncation(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		body := make([]byte, 1<<20+1)
		for i := range body {
			body[i] = ' '
		}
		w.Write(body)
	}))
	defer server.Close()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL, nil)
	if err != nil {
		t.Fatalf("NewRequestWithContext() error = %v", err)
	}
	lookup := newRDAPExpirationLookup(server.Client(), "")
	var target any
	if err := lookup.fetchJSON(req, &target); err == nil {
		t.Fatal("fetchJSON() error = nil, want truncation error")
	}
}

func TestRDAPDomainResponseAcceptsExpirationFallback(t *testing.T) {
	t.Parallel()

	want := time.Date(2031, 2, 3, 4, 5, 6, 0, time.UTC)
	got, err := rdapDomainResponse{
		Events: []rdapEvent{
			{Action: "renewal expiration", Date: want.Format(time.RFC3339)},
		},
	}.expiration()
	if err != nil {
		t.Fatalf("expiration() error = %v, want nil", err)
	}
	if !got.Equal(want) {
		t.Fatalf("expiration() = %v, want %v", got, want)
	}
}

func TestRDAPServiceRejectsMalformedBootstrapEntry(t *testing.T) {
	t.Parallel()

	var service rdapService
	if err := json.Unmarshal([]byte(`[["example"]]`), &service); err == nil {
		t.Fatal("UnmarshalJSON() error = nil, want malformed service error")
	}
}

func TestRDAPServiceRejectsMalformedBootstrapParts(t *testing.T) {
	t.Parallel()

	tests := []string{
		`[{"bad":"tlds"},["https://rdap.example.test"]]`,
		`[["example"],{"bad":"urls"}]`,
	}
	for _, tt := range tests {
		t.Run(tt, func(t *testing.T) {
			t.Parallel()

			var service rdapService
			if err := json.Unmarshal([]byte(tt), &service); err == nil {
				t.Fatal("UnmarshalJSON() error = nil, want malformed service part error")
			}
		})
	}
}

func serverURL(t *testing.T, r *http.Request) string {
	t.Helper()

	return "http://" + r.Host
}

func writeResponse(t *testing.T, w http.ResponseWriter, body string) {
	t.Helper()

	if _, err := fmt.Fprint(w, body); err != nil {
		t.Errorf("write response: %v", err)
	}
}

func writeResponsef(t *testing.T, w http.ResponseWriter, format string, args ...any) {
	t.Helper()

	if _, err := fmt.Fprintf(w, format, args...); err != nil {
		t.Errorf("write response: %v", err)
	}
}
