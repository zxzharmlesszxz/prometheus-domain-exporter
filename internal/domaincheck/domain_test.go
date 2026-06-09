package domaincheck

import (
	"reflect"
	"strings"
	"testing"
)

func TestNormalizeASCII(t *testing.T) {
	t.Parallel()

	got, err := Normalize("Example.COM.")
	if err != nil {
		t.Fatalf("Normalize() error = %v, want nil", err)
	}
	if got != "example.com" {
		t.Fatalf("Normalize() = %q, want %q", got, "example.com")
	}
}

func TestParseDomains(t *testing.T) {
	t.Parallel()

	got, warnings, err := ParseDomains([]string{" Example.COM. ", "example.com", "sub.example.net"})
	if err != nil {
		t.Fatalf("ParseDomains() error = %v, want nil", err)
	}
	want := []string{"example.com", "sub.example.net"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseDomains() = %v, want %v", got, want)
	}
	if len(warnings) != 1 {
		t.Fatalf("ParseDomains() warnings = %v, want 1 modified domain", warnings)
	}
}

func TestParseDomainsDeDuplicates(t *testing.T) {
	t.Parallel()

	got, _, err := ParseDomains([]string{"Example.COM.", "example.com"})
	if err != nil {
		t.Fatalf("ParseDomains() error = %v, want nil", err)
	}
	want := []string{"example.com"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseDomains() = %v, want %v", got, want)
	}
}

func TestNormalizeIDN(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  string
	}{
		{input: "München.de", want: "xn--mnchen-3ya.de"},
		{input: "münchen.de", want: "xn--mnchen-3ya.de"},
		{input: "café.example", want: "xn--caf-dma.example"},
		{input: "例子.测试", want: "xn--fsqu00a.xn--0zwm56d"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()

			got, err := Normalize(tt.input)
			if err != nil {
				t.Fatalf("Normalize() error = %v, want nil", err)
			}
			if got != tt.want {
				t.Fatalf("Normalize() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNormalizeRejectsInvalidValues(t *testing.T) {
	t.Parallel()

	tests := []string{
		"",
		"localhost",
		"https://example.com",
		"example.com:443",
		"-example.com",
		"example-.com",
		"example..com",
		"example.c_m",
		"example." + strings.Repeat("a", 64),
		strings.Repeat("a", 250) + ".com",
	}
	for _, tt := range tests {
		t.Run(tt, func(t *testing.T) {
			t.Parallel()

			if _, err := Normalize(tt); err == nil {
				t.Fatal("Normalize() error = nil, want error")
			}
		})
	}
}

func TestTLD(t *testing.T) {
	t.Parallel()

	if got := tld("sub.example.co.uk"); got != "uk" {
		t.Fatalf("tld() = %q, want %q", got, "uk")
	}
	if got := tld("example"); got != "example" {
		t.Fatalf("tld() = %q, want %q", got, "example")
	}
}
