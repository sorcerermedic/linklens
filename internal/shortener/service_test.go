package shortener

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/portfolio-author/linklens/internal/cache"
	"github.com/portfolio-author/linklens/internal/store"
	"github.com/portfolio-author/linklens/internal/store/memory"
)

type countingStore struct {
	store.Store
	gets atomic.Int64
}

func (c *countingStore) GetLink(ctx context.Context, code string) (store.Link, error) {
	c.gets.Add(1)
	return c.Store.GetLink(ctx, code)
}

type countingObserver struct{ hits, misses atomic.Int64 }

func (o *countingObserver) CacheHit()  { o.hits.Add(1) }
func (o *countingObserver) CacheMiss() { o.misses.Add(1) }

type fixture struct {
	svc   *Service
	store *countingStore
	cache *cache.Memory
	obs   *countingObserver
	now   time.Time
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	f := &fixture{
		store: &countingStore{Store: memory.New()},
		obs:   &countingObserver{},
		now:   time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC),
	}
	clock := func() time.Time { return f.now }
	f.cache = cache.NewMemory(clock)
	f.svc = NewService(f.store, f.cache, Options{CacheTTL: time.Minute, Clock: clock, Observer: f.obs})
	return f
}

func TestService_Create(t *testing.T) {
	ctx := context.Background()
	future := time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)
	past := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name    string
		req     CreateRequest
		wantErr error
		check   func(t *testing.T, l store.Link)
	}{
		{
			name: "generated code",
			req:  CreateRequest{URL: "https://example.com/a"},
			check: func(t *testing.T, l store.Link) {
				if l.Custom || len(l.Code) < 4 || !IsBase62(l.Code) {
					t.Fatalf("unexpected link %+v", l)
				}
			},
		},
		{
			name: "custom alias",
			req:  CreateRequest{URL: "https://example.com/b", Alias: "my-alias"},
			check: func(t *testing.T, l store.Link) {
				if !l.Custom || l.Code != "my-alias" {
					t.Fatalf("unexpected link %+v", l)
				}
			},
		},
		{
			name: "expiry stored in utc",
			req:  CreateRequest{URL: "https://example.com/c", ExpiresAt: &future},
			check: func(t *testing.T, l store.Link) {
				if l.ExpiresAt == nil || !l.ExpiresAt.Equal(future) {
					t.Fatalf("expiry lost: %+v", l)
				}
			},
		},
		{name: "invalid url", req: CreateRequest{URL: "ftp://x.example"}, wantErr: ErrUnsupportedScheme},
		{name: "private url", req: CreateRequest{URL: "http://127.0.0.1/"}, wantErr: ErrPrivateHost},
		{name: "bad alias", req: CreateRequest{URL: "https://example.com", Alias: "a b"}, wantErr: ErrInvalidAlias},
		{name: "reserved alias", req: CreateRequest{URL: "https://example.com", Alias: "metrics"}, wantErr: ErrReservedAlias},
		{name: "expiry in past", req: CreateRequest{URL: "https://example.com", ExpiresAt: &past}, wantErr: ErrInvalidExpiry},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			l, err := f.svc.Create(ctx, tc.req)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("Create error = %v, want %v", err, tc.wantErr)
			}
			if err != nil {
				return
			}
			if !l.CreatedAt.Equal(f.now) {
				t.Fatalf("CreatedAt = %v, want clock time %v", l.CreatedAt, f.now)
			}
			stored, err := f.store.GetLink(ctx, l.Code)
			if err != nil || stored.URL != l.URL {
				t.Fatalf("link not persisted: %+v %v", stored, err)
			}
			tc.check(t, l)
		})
	}
}

func TestService_Create_AliasTaken(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	if _, err := f.svc.Create(ctx, CreateRequest{URL: "https://example.com", Alias: "taken"}); err != nil {
		t.Fatal(err)
	}
	_, err := f.svc.Create(ctx, CreateRequest{URL: "https://example.org", Alias: "taken"})
	if !errors.Is(err, ErrAliasTaken) {
		t.Fatalf("got %v, want ErrAliasTaken", err)
	}
}

func TestService_Create_SkipsCodeOccupiedByAlias(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	if _, err := f.svc.Create(ctx, CreateRequest{URL: "https://example.com", Alias: GenerateCode(2)}); err != nil {
		t.Fatal(err)
	}
	l, err := f.svc.Create(ctx, CreateRequest{URL: "https://example.org"})
	if err != nil {
		t.Fatal(err)
	}
	if l.Code != GenerateCode(3) {
		t.Fatalf("expected retry to pick id 3 (%s), got %s", GenerateCode(3), l.Code)
	}
}

func TestService_Create_GeneratedCodesAreUnique(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	seen := map[string]bool{}
	for i := 0; i < 500; i++ {
		l, err := f.svc.Create(ctx, CreateRequest{URL: "https://example.com/x"})
		if err != nil {
			t.Fatal(err)
		}
		if seen[l.Code] {
			t.Fatalf("duplicate code %s", l.Code)
		}
		seen[l.Code] = true
	}
}

func TestService_Resolve_CacheAside(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	l, err := f.svc.Create(ctx, CreateRequest{URL: "https://example.com/page"})
	if err != nil {
		t.Fatal(err)
	}

	got, err := f.svc.Resolve(ctx, l.Code)
	if err != nil || got.URL != l.URL {
		t.Fatalf("Resolve = %+v, %v", got, err)
	}
	if f.store.gets.Load() != 1 || f.obs.misses.Load() != 1 || f.obs.hits.Load() != 0 {
		t.Fatalf("after first resolve: gets=%d misses=%d hits=%d", f.store.gets.Load(), f.obs.misses.Load(), f.obs.hits.Load())
	}
	if _, err := f.cache.Get(ctx, "link:"+l.Code); err != nil {
		t.Fatalf("cache should be populated: %v", err)
	}

	for i := 0; i < 2; i++ {
		if got, err := f.svc.Resolve(ctx, l.Code); err != nil || got.URL != l.URL {
			t.Fatalf("Resolve #%d = %+v, %v", i+2, got, err)
		}
	}
	if f.store.gets.Load() != 1 || f.obs.hits.Load() != 2 {
		t.Fatalf("after hits: gets=%d hits=%d", f.store.gets.Load(), f.obs.hits.Load())
	}

	f.now = f.now.Add(2 * time.Minute)
	if _, err := f.svc.Resolve(ctx, l.Code); err != nil {
		t.Fatal(err)
	}
	if f.store.gets.Load() != 2 || f.obs.misses.Load() != 2 {
		t.Fatalf("after ttl: gets=%d misses=%d", f.store.gets.Load(), f.obs.misses.Load())
	}
}

func TestService_Resolve_NotFound(t *testing.T) {
	f := newFixture(t)
	_, err := f.svc.Resolve(context.Background(), "nope")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("got %v, want ErrNotFound", err)
	}

	_, _ = f.svc.Resolve(context.Background(), "nope")
	if f.store.gets.Load() != 2 {
		t.Fatalf("gets=%d", f.store.gets.Load())
	}
}

func TestService_Resolve_ExpiredEvenWhenCached(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	exp := f.now.Add(30 * time.Second)
	l, err := f.svc.Create(ctx, CreateRequest{URL: "https://example.com", ExpiresAt: &exp})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Resolve(ctx, l.Code); err != nil {
		t.Fatalf("not yet expired: %v", err)
	}
	f.now = f.now.Add(31 * time.Second)
	got, err := f.svc.Resolve(ctx, l.Code)
	if !errors.Is(err, ErrExpired) {
		t.Fatalf("got %v, want ErrExpired", err)
	}
	if got.URL != l.URL {
		t.Fatalf("expired link should still be returned alongside the error")
	}
	if f.store.gets.Load() != 1 {
		t.Fatalf("expiry must be evaluated from the cached entry, gets=%d", f.store.gets.Load())
	}
}

func TestService_Resolve_CorruptCacheEntry(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	l, _ := f.svc.Create(ctx, CreateRequest{URL: "https://example.com"})
	_ = f.cache.Set(ctx, "link:"+l.Code, []byte("{not json"), time.Minute)
	got, err := f.svc.Resolve(ctx, l.Code)
	if err != nil || got.URL != l.URL {
		t.Fatalf("corrupt cache must fall back to store: %+v %v", got, err)
	}
	raw, err := f.cache.Get(ctx, "link:"+l.Code)
	if err != nil || string(raw) == "{not json" {
		t.Fatalf("corrupt entry should be replaced: %q %v", raw, err)
	}
}

func TestService_Delete_InvalidatesCache(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	l, _ := f.svc.Create(ctx, CreateRequest{URL: "https://example.com"})
	_, _ = f.svc.Resolve(ctx, l.Code)
	if err := f.svc.Delete(ctx, l.Code); err != nil {
		t.Fatal(err)
	}
	if _, err := f.cache.Get(ctx, "link:"+l.Code); !errors.Is(err, cache.ErrMiss) {
		t.Fatalf("cache entry should be gone: %v", err)
	}
	if _, err := f.svc.Resolve(ctx, l.Code); !errors.Is(err, ErrNotFound) {
		t.Fatalf("got %v, want ErrNotFound", err)
	}
	if err := f.svc.Delete(ctx, l.Code); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second delete: %v", err)
	}
}

func TestService_ListAndStats(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	a, _ := f.svc.Create(ctx, CreateRequest{URL: "https://a.example"})
	f.now = f.now.Add(time.Second)
	b, _ := f.svc.Create(ctx, CreateRequest{URL: "https://b.example"})
	_ = f.store.RecordClicks(ctx, []store.Click{{Code: a.Code, At: f.now, Device: "desktop", Country: "US", Referrer: "direct"}})

	list, err := f.svc.List(ctx)
	if err != nil || len(list) != 2 || list[0].Code != b.Code || list[1].Clicks != 1 {
		t.Fatalf("List = %+v, %v", list, err)
	}
	st, err := f.svc.Stats(ctx, a.Code)
	if err != nil || st.TotalClicks != 1 || st.ByDevice["desktop"] != 1 {
		t.Fatalf("Stats = %+v, %v", st, err)
	}
	if _, err := f.svc.Stats(ctx, "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Stats missing = %v", err)
	}
}

func TestGenerateCode(t *testing.T) {
	if got := GenerateCode(0); got != "1000" {
		t.Fatalf("GenerateCode(0) = %q, want %q", got, "1000")
	}
	if got := GenerateCode(1); got != "1001" {
		t.Fatalf("GenerateCode(1) = %q, want %q", got, "1001")
	}
}
