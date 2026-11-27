package demo

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"time"

	"github.com/portfolio-author/linklens/internal/shortener"
	"github.com/portfolio-author/linklens/internal/store"
)

const historyDays = 14

type weighted struct {
	value  string
	weight int
}

type seedLink struct {
	alias      string
	url        string
	ageDays    int
	dailyBase  float64
	trend      float64
	expiredAgo time.Duration
	referrers  []weighted
}

var countries = []weighted{
	{"TR", 68}, {"DE", 10}, {"NL", 5}, {"AZ", 4}, {"US", 4}, {"GB", 3}, {"FR", 3}, {"CY", 3},
}

var devices = []weighted{
	{"mobile", 57}, {"desktop", 34}, {"tablet", 7}, {"bot", 2},
}

var links = []seedLink{
	{
		alias: "kasim-kampanya", url: "https://kahvedukkani.com.tr/kampanyalar/kasim-indirimi",
		ageDays: 13, dailyBase: 38, trend: 0.9,
		referrers: []weighted{{"instagram.com", 46}, {"direct", 20}, {"web.whatsapp.com", 16}, {"x.com", 10}, {"google.com", 8}},
	},
	{
		alias: "devfest-izmir", url: "https://etkinlik.izmirteknoloji.org/devfest-2026/kayit",
		ageDays: 11, dailyBase: 24, trend: 1.4,
		referrers: []weighted{{"linkedin.com", 40}, {"x.com", 22}, {"direct", 18}, {"meetup.com", 12}, {"t.me", 8}},
	},
	{
		url:     "https://blog.ayseyilmaz.dev/go-ile-url-kisaltici-yazmak",
		ageDays: 9, dailyBase: 15, trend: -0.6,
		referrers: []weighted{{"news.ycombinator.com", 30}, {"reddit.com", 22}, {"eksisozluk.com", 18}, {"direct", 16}, {"google.com", 14}},
	},
	{
		alias: "staj-2027", url: "https://kariyer.anadoluyazilim.com.tr/ilanlar/staj-2027",
		ageDays: 7, dailyBase: 18, trend: 0.5,
		referrers: []weighted{{"linkedin.com", 52}, {"kariyer.net", 20}, {"direct", 16}, {"web.whatsapp.com", 12}},
	},
	{
		alias: "menu", url: "https://lezzetduragi-kadikoy.com/menu",
		ageDays: 12, dailyBase: 11, trend: 0.1,
		referrers: []weighted{{"direct", 64}, {"instagram.com", 20}, {"google.com", 16}},
	},
	{
		alias: "kitap-kulubu", url: "https://bulten.ankarakitapkulubu.org/kasim-2026",
		ageDays: 5, dailyBase: 7, trend: 0.3,
		referrers: []weighted{{"web.whatsapp.com", 48}, {"direct", 30}, {"t.me", 22}},
	},
	{
		url:     "https://github.com/portfolio-author/linklens",
		ageDays: 3, dailyBase: 9, trend: 0.8,
		referrers: []weighted{{"github.com", 50}, {"direct", 26}, {"x.com", 24}},
	},
	{
		alias: "veli-toplantisi", url: "https://forms.ornekokul.k12.tr/veli-toplantisi-ekim",
		ageDays: 13, dailyBase: 6, trend: -1, expiredAgo: 6 * 24 * time.Hour,
		referrers: []weighted{{"web.whatsapp.com", 70}, {"direct", 30}},
	},
}

func Seed(ctx context.Context, st store.Store, now time.Time) (int, error) {
	rng := rand.New(rand.NewSource(2026))
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	total := 0
	for i, sl := range links {
		id, err := st.NextID(ctx)
		if err != nil {
			return total, fmt.Errorf("demo: next id: %w", err)
		}
		created := today.AddDate(0, 0, -sl.ageDays).Add(time.Duration(9+i) * time.Hour)
		link := store.Link{ID: id, URL: sl.url, CreatedAt: created, Code: shortener.GenerateCode(id)}
		if sl.alias != "" {
			link.Code, link.Custom = sl.alias, true
		}
		var expires time.Time
		if sl.expiredAgo > 0 {
			expires = now.Add(-sl.expiredAgo)
			link.ExpiresAt = &expires
		}
		if err := st.CreateLink(ctx, link); err != nil {
			return total, fmt.Errorf("demo: create %s: %w", link.Code, err)
		}
		clicks := generateClicks(rng, sl, link.Code, created, expires, today, now)
		if err := st.RecordClicks(ctx, clicks); err != nil {
			return total, fmt.Errorf("demo: record clicks for %s: %w", link.Code, err)
		}
		total += len(clicks)
	}
	return total, nil
}

func generateClicks(rng *rand.Rand, sl seedLink, code string, created, expires, today, now time.Time) []store.Click {
	var out []store.Click
	for d := historyDays - 1; d >= 0; d-- {
		day := today.AddDate(0, 0, -d)
		if day.Before(created.Truncate(24 * time.Hour)) {
			continue
		}
		age := day.Sub(created).Hours() / 24
		level := sl.dailyBase * (1 + sl.trend*age/float64(historyDays))
		if wd := day.Weekday(); wd == time.Saturday || wd == time.Sunday {
			level *= 1.25
		}
		level *= 0.8 + rng.Float64()*0.4
		n := int(math.Max(0, math.Round(level)))
		for k := 0; k < n; k++ {
			at := day.Add(time.Duration(7*3600+rng.Intn(16*3600)) * time.Second)
			if at.Before(created) || at.After(now) || (!expires.IsZero() && !at.Before(expires)) {
				continue
			}
			out = append(out, store.Click{
				Code:     code,
				At:       at,
				Country:  pick(rng, countries),
				Device:   pick(rng, devices),
				Referrer: pick(rng, sl.referrers),
			})
		}
	}
	return out
}

func pick(rng *rand.Rand, options []weighted) string {
	sum := 0
	for _, o := range options {
		sum += o.weight
	}
	r := rng.Intn(sum)
	for _, o := range options {
		if r < o.weight {
			return o.value
		}
		r -= o.weight
	}
	return options[len(options)-1].value
}
