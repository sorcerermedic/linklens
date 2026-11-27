//go:build integration

package postgres

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/portfolio-author/linklens/internal/store"
)

func TestPostgresStore(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if os.Getenv("INTEGRATION") == "" || dsn == "" {
		t.Skip("set INTEGRATION=1 and DATABASE_URL to run")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	s, err := Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := s.pool.Exec(ctx, `TRUNCATE clicks, links`); err != nil {
		t.Fatal(err)
	}

	id1, err := s.NextID(ctx)
	if err != nil {
		t.Fatal(err)
	}
	id2, _ := s.NextID(ctx)
	if id2 <= id1 {
		t.Fatalf("ids must increase: %d, %d", id1, id2)
	}

	now := time.Now().UTC().Truncate(time.Microsecond)
	exp := now.Add(time.Hour)
	l1 := store.Link{ID: id1, Code: "itest1", URL: "https://a.example", CreatedAt: now}
	l2 := store.Link{ID: id2, Code: "itest2", URL: "https://b.example", Custom: true, CreatedAt: now.Add(time.Second), ExpiresAt: &exp}
	for _, l := range []store.Link{l1, l2} {
		if err := s.CreateLink(ctx, l); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.CreateLink(ctx, store.Link{ID: id1 + 1000, Code: "itest1", URL: "x", CreatedAt: now}); !errors.Is(err, store.ErrCodeTaken) {
		t.Fatalf("duplicate: got %v", err)
	}

	got, err := s.GetLink(ctx, "itest2")
	if err != nil {
		t.Fatal(err)
	}
	if got.URL != l2.URL || !got.Custom || got.ExpiresAt == nil || !got.ExpiresAt.Equal(exp) {
		t.Fatalf("GetLink = %+v", got)
	}
	if _, err := s.GetLink(ctx, "missing"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("missing: got %v", err)
	}

	clicks := []store.Click{
		{Code: "itest1", At: now, Country: "US", Device: "desktop", Referrer: "direct"},
		{Code: "itest1", At: now.Add(24 * time.Hour), Country: "TR", Device: "mobile", Referrer: "news.example"},
		{Code: "itest1", At: now.Add(25 * time.Hour), Country: "TR", Device: "mobile", Referrer: "news.example"},
		{Code: "missing", At: now},
	}
	if err := s.RecordClicks(ctx, clicks); err != nil {
		t.Fatal(err)
	}

	st, err := s.Stats(ctx, "itest1")
	if err != nil {
		t.Fatal(err)
	}
	if st.TotalClicks != 3 || len(st.ByDay) != 2 || st.ByDevice["mobile"] != 2 || st.ByCountry["TR"] != 2 {
		t.Fatalf("stats = %+v", st)
	}
	if len(st.TopReferrers) != 2 || st.TopReferrers[0].Referrer != "news.example" {
		t.Fatalf("referrers = %+v", st.TopReferrers)
	}
	if _, err := s.Stats(ctx, "missing"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("stats missing: got %v", err)
	}

	list, err := s.ListLinks(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || list[0].Code != "itest2" || list[1].Clicks != 3 {
		t.Fatalf("list = %+v", list)
	}

	if err := s.DeleteLink(ctx, "itest1"); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteLink(ctx, "itest1"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("second delete: got %v", err)
	}
	if err := s.Ping(ctx); err != nil {
		t.Fatal(err)
	}
}
