package memory

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/portfolio-author/linklens/internal/store"
)

func TestStore_CRUD(t *testing.T) {
	ctx := context.Background()
	s := New()

	id1, _ := s.NextID(ctx)
	id2, _ := s.NextID(ctx)
	if id2 <= id1 {
		t.Fatalf("ids must increase: %d then %d", id1, id2)
	}

	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	l1 := store.Link{ID: id1, Code: "aaaa", URL: "https://a.example", CreatedAt: now}
	l2 := store.Link{ID: id2, Code: "bbbb", URL: "https://b.example", CreatedAt: now.Add(time.Minute)}
	if err := s.CreateLink(ctx, l1); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateLink(ctx, l2); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateLink(ctx, l1); !errors.Is(err, store.ErrCodeTaken) {
		t.Fatalf("duplicate code: got %v, want ErrCodeTaken", err)
	}

	got, err := s.GetLink(ctx, "aaaa")
	if err != nil || got.URL != l1.URL {
		t.Fatalf("GetLink = %+v, %v", got, err)
	}
	if _, err := s.GetLink(ctx, "zzzz"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("missing link: got %v", err)
	}

	if err := s.RecordClicks(ctx, []store.Click{
		{Code: "aaaa", At: now, Device: "desktop", Country: "US", Referrer: "direct"},
		{Code: "aaaa", At: now.Add(24 * time.Hour), Device: "mobile", Country: "TR", Referrer: "news.example"},
		{Code: "nope", At: now},
	}); err != nil {
		t.Fatal(err)
	}

	list, err := s.ListLinks(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || list[0].Code != "bbbb" || list[1].Code != "aaaa" {
		t.Fatalf("ListLinks order wrong: %+v", list)
	}
	if list[1].Clicks != 2 {
		t.Fatalf("clicks not populated: %+v", list[1])
	}

	st, err := s.Stats(ctx, "aaaa")
	if err != nil {
		t.Fatal(err)
	}
	if st.TotalClicks != 2 || len(st.ByDay) != 2 || st.ByDay[0].Day != "2026-01-02" || st.ByDay[1].Day != "2026-01-03" {
		t.Fatalf("unexpected stats: %+v", st)
	}
	if st.ByDevice["desktop"] != 1 || st.ByDevice["mobile"] != 1 || st.ByCountry["TR"] != 1 {
		t.Fatalf("unexpected breakdowns: %+v", st)
	}
	if _, err := s.Stats(ctx, "zzzz"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("stats of missing link: got %v", err)
	}

	if err := s.DeleteLink(ctx, "aaaa"); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteLink(ctx, "aaaa"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("second delete: got %v", err)
	}
	if _, err := s.GetLink(ctx, "aaaa"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("deleted link still present: %v", err)
	}
}

func TestAggregate_TopReferrers(t *testing.T) {
	now := time.Now()
	var clicks []store.Click
	for i := 0; i < 15; i++ {

		for j := 0; j <= i; j++ {
			clicks = append(clicks, store.Click{Code: "x", At: now, Referrer: fmt.Sprintf("ref%02d", i)})
		}
	}
	st := Aggregate(clicks)
	if len(st.TopReferrers) != store.TopReferrersLimit {
		t.Fatalf("len(TopReferrers) = %d, want %d", len(st.TopReferrers), store.TopReferrersLimit)
	}
	if st.TopReferrers[0].Referrer != "ref14" || st.TopReferrers[0].Clicks != 15 {
		t.Fatalf("top referrer wrong: %+v", st.TopReferrers[0])
	}
	for i := 1; i < len(st.TopReferrers); i++ {
		if st.TopReferrers[i].Clicks > st.TopReferrers[i-1].Clicks {
			t.Fatalf("referrers not sorted desc: %+v", st.TopReferrers)
		}
	}
}

func TestAggregate_Empty(t *testing.T) {
	st := Aggregate(nil)
	if st.TotalClicks != 0 || st.ByDay == nil || st.ByDevice == nil || st.ByCountry == nil || st.TopReferrers == nil {
		t.Fatalf("empty aggregate must have non-nil collections: %+v", st)
	}
}

func TestLink_Expired(t *testing.T) {
	now := time.Now()
	past := now.Add(-time.Second)
	future := now.Add(time.Second)
	tests := []struct {
		name string
		link store.Link
		want bool
	}{
		{"no expiry", store.Link{}, false},
		{"future", store.Link{ExpiresAt: &future}, false},
		{"past", store.Link{ExpiresAt: &past}, true},
		{"exactly now", store.Link{ExpiresAt: &now}, true},
	}
	for _, tc := range tests {
		if got := tc.link.Expired(now); got != tc.want {
			t.Errorf("%s: Expired = %v, want %v", tc.name, got, tc.want)
		}
	}
}
