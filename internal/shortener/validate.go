package shortener

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strings"
)

var (
	ErrInvalidURL = errors.New("invalid url")

	ErrUnsupportedScheme = errors.New("only http and https urls are allowed")

	ErrPrivateHost = errors.New("urls pointing at localhost or private networks are not allowed")

	ErrURLTooLong = errors.New("url is too long")

	ErrInvalidAlias = errors.New("alias must be 4-32 characters of letters, digits, '-' or '_'")

	ErrReservedAlias = errors.New("alias is reserved")
)

const DefaultMaxURLLength = 2048

type URLValidator struct {
	AllowPrivate bool

	MaxLength int
}

func (v URLValidator) Validate(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("%w: empty", ErrInvalidURL)
	}
	maxLen := v.MaxLength
	if maxLen <= 0 {
		maxLen = DefaultMaxURLLength
	}
	if len(raw) > maxLen {
		return "", ErrURLTooLong
	}

	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrInvalidURL, err)
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return "", ErrUnsupportedScheme
	}
	u.Scheme = scheme
	if u.Host == "" || u.Hostname() == "" {
		return "", fmt.Errorf("%w: missing host", ErrInvalidURL)
	}
	if u.User != nil {
		return "", fmt.Errorf("%w: credentials in url are not allowed", ErrInvalidURL)
	}
	if !v.AllowPrivate {
		if err := checkPublicHost(u.Hostname()); err != nil {
			return "", err
		}
	}
	u.Host = strings.ToLower(u.Host)
	return u.String(), nil
}

func checkPublicHost(host string) error {
	h := strings.ToLower(strings.TrimSuffix(host, "."))
	if h == "localhost" || strings.HasSuffix(h, ".localhost") || strings.HasSuffix(h, ".local") || strings.HasSuffix(h, ".internal") {
		return ErrPrivateHost
	}
	if ip := net.ParseIP(h); ip != nil {
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
			ip.IsLinkLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified() {
			return ErrPrivateHost
		}
		return nil
	}

	if !strings.Contains(h, ".") {
		return ErrPrivateHost
	}
	return nil
}

var aliasPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{4,32}$`)

var reservedAliases = map[string]struct{}{
	"api": {}, "app": {}, "healthz": {}, "metrics": {}, "static": {}, "assets": {},
}

func ValidateAlias(alias string) error {
	if !aliasPattern.MatchString(alias) {
		return ErrInvalidAlias
	}
	if _, reserved := reservedAliases[strings.ToLower(alias)]; reserved {
		return ErrReservedAlias
	}
	return nil
}

func IsValidCode(s string) bool {
	return aliasPattern.MatchString(s) || (IsBase62(s) && len(s) <= 11)
}
