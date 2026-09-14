package domaincheck

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestWHOISExpirationLookup(t *testing.T) {
	t.Parallel()

	want := time.Date(2031, 8, 28, 4, 11, 24, 0, time.UTC)
	bootstrapQueries := 0
	lookup := newWHOISExpirationLookup(func(_ context.Context, server, query string) (string, error) {
		switch {
		case server == defaultWHOISBootstrapServer && query == "ws":
			bootstrapQueries++
			return "domain: WS\nwhois: whois.website.ws\n", nil
		case server == "whois.website.ws":
			return "Domain Name: " + query + "\nRegistrar Registration Expiration Date: " + want.Format(time.RFC3339) + "\n", nil
		default:
			return "", fmt.Errorf("unexpected WHOIS query %s to %s", query, server)
		}
	}, "")

	for _, name := range []string{"example.ws", "second.ws"} {
		got, verified, err := lookup.LookupExpiration(context.Background(), name)
		if err != nil {
			t.Fatalf("LookupExpiration(%q) error = %v, want nil", name, err)
		}
		if !verified {
			t.Fatalf("LookupExpiration(%q) verified = false, want true", name)
		}
		if !got.Equal(want) {
			t.Fatalf("LookupExpiration(%q) = %v, want %v", name, got, want)
		}
	}
	if bootstrapQueries != 1 {
		t.Fatalf("WHOIS bootstrap queries = %d, want 1", bootstrapQueries)
	}
}

func TestWHOISExpirationLookupReportsNotFound(t *testing.T) {
	t.Parallel()

	lookup := newWHOISExpirationLookup(func(_ context.Context, server, _ string) (string, error) {
		if server == defaultWHOISBootstrapServer {
			return "whois: whois.example.test\n", nil
		}
		return "No match for domain EXAMPLE.TEST\n", nil
	}, "")

	expiration, verified, err := lookup.LookupExpiration(context.Background(), "example.test")
	if err != nil {
		t.Fatalf("LookupExpiration() error = %v, want nil", err)
	}
	if verified {
		t.Fatal("LookupExpiration() verified = true, want false")
	}
	if !expiration.IsZero() {
		t.Fatalf("LookupExpiration() expiration = %v, want zero", expiration)
	}
}

func TestWHOISExpirationLookupReportsMissingService(t *testing.T) {
	t.Parallel()

	lookup := newWHOISExpirationLookup(func(context.Context, string, string) (string, error) {
		return "domain: TEST\n", nil
	}, "")
	if _, _, err := lookup.LookupExpiration(context.Background(), "example.test"); err == nil {
		t.Fatal("LookupExpiration() error = nil, want missing WHOIS service error")
	}
}

func TestParseWHOISExpiration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		response string
		want     time.Time
	}{
		{
			name:     "registry expiry",
			response: "Registry Expiry Date: 2030-01-02T03:04:05Z\n",
			want:     time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC),
		},
		{
			name:     "date only",
			response: "Expiry Date: 2030-01-02\n",
			want:     time.Date(2030, 1, 2, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "text month",
			response: "Expiration Date: 02-Jan-2030\n",
			want:     time.Date(2030, 1, 2, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "registry field priority",
			response: "Expiration Date: 2029-01-02\nRegistry Expiry Date: 2030-01-02T03:04:05Z\n",
			want:     time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, verified, err := parseWHOISExpiration(tt.response)
			if err != nil {
				t.Fatalf("parseWHOISExpiration() error = %v, want nil", err)
			}
			if !verified || !got.Equal(tt.want) {
				t.Fatalf("parseWHOISExpiration() = %v, %v, want %v, true", got, verified, tt.want)
			}
		})
	}
}

func TestParseWHOISExpirationReportsMalformedDate(t *testing.T) {
	t.Parallel()

	_, verified, err := parseWHOISExpiration("Expiration Date: tomorrow\n")
	if err == nil {
		t.Fatal("parseWHOISExpiration() error = nil, want malformed date error")
	}
	if !IsLookupParseError(err) {
		t.Fatalf("parseWHOISExpiration() error = %v, want lookup parse error", err)
	}
	if !verified {
		t.Fatal("parseWHOISExpiration() verified = false, want true")
	}
}

func TestWHOISAddress(t *testing.T) {
	t.Parallel()

	if got := whoisAddress("whois://whois.example.test/"); got != "whois.example.test:43" {
		t.Fatalf("whoisAddress() = %q, want %q", got, "whois.example.test:43")
	}
	if got := whoisAddress("localhost:4343"); got != "localhost:4343" {
		t.Fatalf("whoisAddress() = %q, want existing port", got)
	}
}

func TestQueryWHOISRetriesWithinTimeout(t *testing.T) {
	t.Parallel()

	const timeout = 10 * time.Second
	attempts := 0
	response, err := queryWHOISWithRetry(context.Background(), timeout, "whois.example.test", "example.test", func(_ context.Context, gotTimeout time.Duration, server, query string) (string, error) {
		attempts++
		if gotTimeout != timeout/2 {
			t.Fatalf("attempt timeout = %s, want %s", gotTimeout, timeout/2)
		}
		if server != "whois.example.test" || query != "example.test" {
			t.Fatalf("WHOIS query = %q to %q", query, server)
		}
		if attempts == 1 {
			return "", fmt.Errorf("dial timeout")
		}
		return "response", nil
	})
	if err != nil {
		t.Fatalf("queryWHOISWithRetry() error = %v, want nil", err)
	}
	if response != "response" || attempts != 2 {
		t.Fatalf("queryWHOISWithRetry() = %q after %d attempts, want response after 2", response, attempts)
	}
}
