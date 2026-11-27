package shortener

import (
	"errors"
	"strings"
	"testing"
)

func TestURLValidator_Validate(t *testing.T) {
	tests := []struct {
		name         string
		in           string
		allowPrivate bool
		want         string
		wantErr      error
	}{
		{name: "plain https", in: "https://example.com/path?q=1", want: "https://example.com/path?q=1"},
		{name: "plain http", in: "http://example.com", want: "http://example.com"},
		{name: "trims whitespace", in: "  https://example.com  ", want: "https://example.com"},
		{name: "uppercase scheme and host normalised", in: "HTTPS://Example.COM/Path", want: "https://example.com/Path"},
		{name: "public ip", in: "http://93.184.216.34/", want: "http://93.184.216.34/"},
		{name: "with port", in: "https://example.com:8443/x", want: "https://example.com:8443/x"},
		{name: "empty", in: "", wantErr: ErrInvalidURL},
		{name: "no scheme", in: "example.com", wantErr: ErrUnsupportedScheme},
		{name: "ftp", in: "ftp://example.com/file", wantErr: ErrUnsupportedScheme},
		{name: "javascript", in: "javascript:alert(1)", wantErr: ErrUnsupportedScheme},
		{name: "missing host", in: "https:///path", wantErr: ErrInvalidURL},
		{name: "userinfo", in: "https://user:pass@example.com", wantErr: ErrInvalidURL},
		{name: "localhost", in: "http://localhost:8080/admin", wantErr: ErrPrivateHost},
		{name: "localhost subdomain", in: "http://foo.localhost/", wantErr: ErrPrivateHost},
		{name: "dot local", in: "http://printer.local/", wantErr: ErrPrivateHost},
		{name: "loopback ipv4", in: "http://127.0.0.1/", wantErr: ErrPrivateHost},
		{name: "loopback ipv6", in: "http://[::1]/", wantErr: ErrPrivateHost},
		{name: "private 10/8", in: "http://10.0.0.5/", wantErr: ErrPrivateHost},
		{name: "private 172.16/12", in: "http://172.16.5.4/", wantErr: ErrPrivateHost},
		{name: "private 192.168/16", in: "http://192.168.1.1/", wantErr: ErrPrivateHost},
		{name: "link local", in: "http://169.254.169.254/latest/meta-data", wantErr: ErrPrivateHost},
		{name: "unspecified", in: "http://0.0.0.0/", wantErr: ErrPrivateHost},
		{name: "single label host", in: "http://intranet/", wantErr: ErrPrivateHost},
		{name: "private allowed", in: "http://localhost:3000/", allowPrivate: true, want: "http://localhost:3000/"},
		{name: "too long", in: "https://example.com/" + strings.Repeat("a", 3000), wantErr: ErrURLTooLong},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			v := URLValidator{AllowPrivate: tc.allowPrivate}
			got, err := v.Validate(tc.in)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("Validate(%q) error = %v, want %v", tc.in, err, tc.wantErr)
			}
			if err == nil && got != tc.want {
				t.Fatalf("Validate(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestURLValidator_MaxLength(t *testing.T) {
	v := URLValidator{MaxLength: 30}
	if _, err := v.Validate("https://example.com/" + strings.Repeat("x", 20)); !errors.Is(err, ErrURLTooLong) {
		t.Fatalf("expected ErrURLTooLong, got %v", err)
	}
}

func TestValidateAlias(t *testing.T) {
	tests := []struct {
		alias   string
		wantErr error
	}{
		{"my-link", nil},
		{"My_Link_2026", nil},
		{"abcd", nil},
		{"abc", ErrInvalidAlias},
		{"", ErrInvalidAlias},
		{"has space", ErrInvalidAlias},
		{"has/slash", ErrInvalidAlias},
		{strings.Repeat("a", 33), ErrInvalidAlias},
		{"api", ErrInvalidAlias},
		{"healthz", ErrReservedAlias},
		{"metrics", ErrReservedAlias},
		{"Metrics", ErrReservedAlias},
	}
	for _, tc := range tests {
		t.Run(tc.alias, func(t *testing.T) {
			if err := ValidateAlias(tc.alias); !errors.Is(err, tc.wantErr) {
				t.Fatalf("ValidateAlias(%q) = %v, want %v", tc.alias, err, tc.wantErr)
			}
		})
	}
}

func TestIsValidCode(t *testing.T) {
	tests := map[string]bool{
		"1000":                                true,
		"aZ9":                                 true,
		"my-alias":                            true,
		"":                                    false,
		"a b":                                 false,
		"../etc":                              false,
		"ab":                                  true,
		"x_y":                                 false,
		"toolongtoolongtoolongtoolongtoolong": false,
	}
	for in, want := range tests {
		if got := IsValidCode(in); got != want {
			t.Errorf("IsValidCode(%q) = %v, want %v", in, got, want)
		}
	}
}
