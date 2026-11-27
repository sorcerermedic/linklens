package store

import (
	"context"
	"errors"
	"time"
)

var (
	ErrNotFound = errors.New("link not found")

	ErrCodeTaken = errors.New("code already taken")
)

const TopReferrersLimit = 10

type Link struct {
	ID        int64      `json:"-"`
	Code      string     `json:"code"`
	URL       string     `json:"url"`
	Custom    bool       `json:"custom"`
	CreatedAt time.Time  `json:"createdAt"`
	ExpiresAt *time.Time `json:"expiresAt,omitempty"`

	Clicks int64 `json:"clicks"`
}

func (l Link) Expired(now time.Time) bool {
	return l.ExpiresAt != nil && !now.Before(*l.ExpiresAt)
}

type Click struct {
	Code     string
	At       time.Time
	Country  string
	Device   string
	Referrer string
}

type DayCount struct {
	Day    string `json:"day"`
	Clicks int64  `json:"clicks"`
}

type ReferrerCount struct {
	Referrer string `json:"referrer"`
	Clicks   int64  `json:"clicks"`
}

type Stats struct {
	TotalClicks  int64            `json:"totalClicks"`
	ByDay        []DayCount       `json:"byDay"`
	ByDevice     map[string]int64 `json:"byDevice"`
	ByCountry    map[string]int64 `json:"byCountry"`
	TopReferrers []ReferrerCount  `json:"topReferrers"`
}

type Store interface {
	NextID(ctx context.Context) (int64, error)

	CreateLink(ctx context.Context, link Link) error

	GetLink(ctx context.Context, code string) (Link, error)

	ListLinks(ctx context.Context) ([]Link, error)

	DeleteLink(ctx context.Context, code string) error

	RecordClicks(ctx context.Context, clicks []Click) error

	Stats(ctx context.Context, code string) (Stats, error)

	Ping(ctx context.Context) error

	Close()
}

func DayKey(t time.Time) string {
	return t.UTC().Format("2006-01-02")
}
