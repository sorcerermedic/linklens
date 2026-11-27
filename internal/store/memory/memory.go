package memory

import (
	"context"
	"sort"
	"sync"

	"github.com/portfolio-author/linklens/internal/store"
)

type Store struct {
	mu     sync.RWMutex
	nextID int64
	links  map[string]store.Link
	clicks map[string][]store.Click
}

func New() *Store {
	return &Store{
		links:  make(map[string]store.Link),
		clicks: make(map[string][]store.Click),
	}
}

func (s *Store) NextID(_ context.Context) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextID++
	return s.nextID, nil
}

func (s *Store) CreateLink(_ context.Context, link store.Link) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.links[link.Code]; exists {
		return store.ErrCodeTaken
	}
	s.links[link.Code] = link
	return nil
}

func (s *Store) GetLink(_ context.Context, code string) (store.Link, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	link, ok := s.links[code]
	if !ok {
		return store.Link{}, store.ErrNotFound
	}
	return link, nil
}

func (s *Store) ListLinks(_ context.Context) ([]store.Link, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]store.Link, 0, len(s.links))
	for _, l := range s.links {
		l.Clicks = int64(len(s.clicks[l.Code]))
		out = append(out, l)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].ID > out[j].ID
		}
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	return out, nil
}

func (s *Store) DeleteLink(_ context.Context, code string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.links[code]; !ok {
		return store.ErrNotFound
	}
	delete(s.links, code)
	delete(s.clicks, code)
	return nil
}

func (s *Store) RecordClicks(_ context.Context, clicks []store.Click) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, c := range clicks {
		if _, ok := s.links[c.Code]; !ok {
			continue
		}
		s.clicks[c.Code] = append(s.clicks[c.Code], c)
	}
	return nil
}

func (s *Store) Stats(_ context.Context, code string) (store.Stats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if _, ok := s.links[code]; !ok {
		return store.Stats{}, store.ErrNotFound
	}
	return Aggregate(s.clicks[code]), nil
}

func (s *Store) Ping(context.Context) error { return nil }

func (s *Store) Close() {}

func Aggregate(clicks []store.Click) store.Stats {
	st := store.Stats{
		TotalClicks:  int64(len(clicks)),
		ByDay:        []store.DayCount{},
		ByDevice:     map[string]int64{},
		ByCountry:    map[string]int64{},
		TopReferrers: []store.ReferrerCount{},
	}
	days := map[string]int64{}
	refs := map[string]int64{}
	for _, c := range clicks {
		days[store.DayKey(c.At)]++
		st.ByDevice[c.Device]++
		st.ByCountry[c.Country]++
		refs[c.Referrer]++
	}
	for day, n := range days {
		st.ByDay = append(st.ByDay, store.DayCount{Day: day, Clicks: n})
	}
	sort.Slice(st.ByDay, func(i, j int) bool { return st.ByDay[i].Day < st.ByDay[j].Day })

	for ref, n := range refs {
		st.TopReferrers = append(st.TopReferrers, store.ReferrerCount{Referrer: ref, Clicks: n})
	}
	sort.Slice(st.TopReferrers, func(i, j int) bool {
		if st.TopReferrers[i].Clicks == st.TopReferrers[j].Clicks {
			return st.TopReferrers[i].Referrer < st.TopReferrers[j].Referrer
		}
		return st.TopReferrers[i].Clicks > st.TopReferrers[j].Clicks
	})
	if len(st.TopReferrers) > store.TopReferrersLimit {
		st.TopReferrers = st.TopReferrers[:store.TopReferrersLimit]
	}
	return st
}
