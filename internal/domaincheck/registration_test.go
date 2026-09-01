package domaincheck

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRegistrationExpirationLookupFallsBackToWHOIS(t *testing.T) {
	t.Parallel()

	rdapServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeResponse(t, w, `{"services":[[["com"],["https://rdap.example.test/"]]]}`)
	}))
	defer rdapServer.Close()

	want := time.Date(2031, 8, 28, 4, 11, 24, 0, time.UTC)
	whois := newWHOISExpirationLookup(func(_ context.Context, server, query string) (string, error) {
		switch {
		case server == defaultWHOISBootstrapServer && query == "ws":
			return "whois: whois.website.ws\n", nil
		case server == "whois.website.ws" && query == "example.ws":
			return "Registrar Registration Expiration Date: " + want.Format(time.RFC3339) + "\n", nil
		default:
			return "", fmt.Errorf("unexpected WHOIS query %s to %s", query, server)
		}
	}, "")
	lookup := &RegistrationExpirationLookup{
		RDAP:  newRDAPExpirationLookup(rdapServer.Client(), rdapServer.URL),
		WHOIS: whois,
	}

	got, verified, source, err := lookup.LookupExpirationSource(context.Background(), "example.ws")
	if err != nil {
		t.Fatalf("LookupExpirationSource() error = %v, want nil", err)
	}
	if !verified {
		t.Fatal("LookupExpirationSource() verified = false, want true")
	}
	if source != SourceWHOIS {
		t.Fatalf("LookupExpirationSource() source = %q, want %q", source, SourceWHOIS)
	}
	if !got.Equal(want) {
		t.Fatalf("LookupExpirationSource() expiration = %v, want %v", got, want)
	}
}

func TestRegistrationExpirationLookupDoesNotMaskRDAPFailure(t *testing.T) {
	t.Parallel()

	rdapServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/dns.json":
			writeResponsef(t, w, `{"services":[[["example"],["%s/rdap/"]]]}`, serverURL(t, r))
		case "/rdap/domain/example.example":
			http.Error(w, "registry unavailable", http.StatusServiceUnavailable)
		default:
			http.NotFound(w, r)
		}
	}))
	defer rdapServer.Close()

	whoisCalled := false
	lookup := &RegistrationExpirationLookup{
		RDAP: newRDAPExpirationLookup(rdapServer.Client(), rdapServer.URL+"/dns.json"),
		WHOIS: newWHOISExpirationLookup(func(context.Context, string, string) (string, error) {
			whoisCalled = true
			return "", nil
		}, ""),
	}

	_, _, source, err := lookup.LookupExpirationSource(context.Background(), "example.example")
	if err == nil {
		t.Fatal("LookupExpirationSource() error = nil, want RDAP HTTP error")
	}
	if source != SourceRDAP {
		t.Fatalf("LookupExpirationSource() source = %q, want %q", source, SourceRDAP)
	}
	if whoisCalled {
		t.Fatal("WHOIS fallback called for an RDAP service failure")
	}
}
