package analytics

import "net"

const CountryUnknown = "unknown"

type geoEntry struct {
	cidr    *net.IPNet
	country string
}

var geoTable = mustBuildGeoTable(map[string]string{
	"8.8.8.0/24":      "US",
	"8.8.4.0/24":      "US",
	"1.1.1.0/24":      "AU",
	"1.0.0.0/24":      "AU",
	"9.9.9.0/24":      "US",
	"208.67.222.0/24": "US",
	"77.88.8.0/24":    "RU",
	"195.175.39.0/24": "TR",
	"2001:4860::/32":  "US",
	"2606:4700::/32":  "US",
})

func mustBuildGeoTable(src map[string]string) []geoEntry {
	out := make([]geoEntry, 0, len(src))
	for cidr, country := range src {
		_, n, err := net.ParseCIDR(cidr)
		if err != nil {
			panic("analytics: bad geo table entry " + cidr + ": " + err.Error())
		}
		out = append(out, geoEntry{cidr: n, country: country})
	}
	return out
}

func LookupCountry(hint, ip string) string {
	if h := normalizeCountry(hint); h != "" {
		return h
	}
	parsed := net.ParseIP(ip)
	if parsed == nil || parsed.IsPrivate() || parsed.IsLoopback() || parsed.IsUnspecified() {
		return CountryUnknown
	}
	for _, e := range geoTable {
		if e.cidr.Contains(parsed) {
			return e.country
		}
	}
	return CountryUnknown
}

func normalizeCountry(s string) string {
	if len(s) != 2 {
		return ""
	}
	var out [2]byte
	for i := 0; i < 2; i++ {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z':
			c -= 'a' - 'A'
		case c >= 'A' && c <= 'Z':
		default:
			return ""
		}
		out[i] = c
	}
	if out == [2]byte{'X', 'X'} || out == [2]byte{'T', '1'} {
		return ""
	}
	return string(out[:])
}
