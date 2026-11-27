package analytics

import (
	"net/url"
	"strings"
	"time"

	"github.com/portfolio-author/linklens/internal/store"
)

const ReferrerDirect = "direct"

type Event struct {
	Code        string
	At          time.Time
	IP          string
	UserAgent   string
	Referrer    string
	CountryHint string
}

func Enrich(ev Event) store.Click {
	return store.Click{
		Code:     ev.Code,
		At:       ev.At.UTC(),
		Country:  LookupCountry(ev.CountryHint, ev.IP),
		Device:   ClassifyDevice(ev.UserAgent),
		Referrer: NormalizeReferrer(ev.Referrer),
	}
}

func NormalizeReferrer(ref string) string {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return ReferrerDirect
	}
	u, err := url.Parse(ref)
	if err != nil || u.Hostname() == "" {
		return ReferrerDirect
	}
	host := strings.ToLower(u.Hostname())
	return strings.TrimPrefix(host, "www.")
}
