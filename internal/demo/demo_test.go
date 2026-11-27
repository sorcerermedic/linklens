package demo

import (
	"context"
	"testing"
	"time"

	"github.com/portfolio-author/linklens/internal/store/memory"
)

func TestSeed(t *testing.T) {
	ctx := context.Background()
	st := memory.New()
	now := time.Date(2026, 11, 27, 15, 0, 0, 0, time.UTC)

	total, err := Seed(ctx, st, now)
	if err != nil {
		t.Fatal(err)
	}
	if total < 500 {
		t.Fatalf("expected a few hundred clicks, got %d", total)
	}

	list, err := st.ListLinks(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != len(links) {
		t.Fatalf("links: got %d want %d", len(list), len(links))
	}
	var sum int64
	for _, l := range list {
		sum += l.Clicks
		if l.CreatedAt.After(now) {
			t.Fatalf("%s created in the future", l.Code)
		}
	}
	if sum != int64(total) {
		t.Fatalf("click totals %d != recorded %d", sum, total)
	}

	expired, err := st.GetLink(ctx, "veli-toplantisi")
	if err != nil {
		t.Fatal(err)
	}
	if !expired.Expired(now) {
		t.Fatal("veli-toplantisi should be expired")
	}

	stats, err := st.Stats(ctx, "kasim-kampanya")
	if err != nil {
		t.Fatal(err)
	}
	if len(stats.ByDay) < 10 || len(stats.ByDay) > historyDays {
		t.Fatalf("byDay length %d", len(stats.ByDay))
	}
	if stats.ByCountry["TR"] == 0 || stats.ByDevice["mobile"] == 0 || len(stats.TopReferrers) == 0 {
		t.Fatalf("stats look empty: %+v", stats)
	}
}

func TestSeedIsDeterministic(t *testing.T) {
	now := time.Date(2026, 11, 27, 15, 0, 0, 0, time.UTC)
	a, err := Seed(context.Background(), memory.New(), now)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Seed(context.Background(), memory.New(), now)
	if err != nil {
		t.Fatal(err)
	}
	if a != b {
		t.Fatalf("seed not deterministic: %d vs %d", a, b)
	}
}

func TestSeedRejectsExistingCodes(t *testing.T) {
	st := memory.New()
	now := time.Now()
	if _, err := Seed(context.Background(), st, now); err != nil {
		t.Fatal(err)
	}
	if _, err := Seed(context.Background(), st, now); err == nil {
		t.Fatal("second seed into the same store should fail on duplicate aliases")
	}
}
