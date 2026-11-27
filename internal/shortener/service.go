package shortener

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/portfolio-author/linklens/internal/cache"
	"github.com/portfolio-author/linklens/internal/store"
)

const idOffset = 62 * 62 * 62

const maxGenerateAttempts = 5

const DefaultCacheTTL = 10 * time.Minute

var (
	ErrExpired = errors.New("link has expired")

	ErrInvalidExpiry = errors.New("expiresAt must be in the future")

	ErrAliasTaken = errors.New("alias already taken")

	ErrNotFound = store.ErrNotFound
)

type CacheObserver interface {
	CacheHit()
	CacheMiss()
}

type noopObserver struct{}

func (noopObserver) CacheHit()  {}
func (noopObserver) CacheMiss() {}

type Options struct {
	Validator URLValidator
	CacheTTL  time.Duration
	Clock     func() time.Time
	Observer  CacheObserver
	Logger    *slog.Logger
}

type Service struct {
	store     store.Store
	cache     cache.Cache
	validator URLValidator
	ttl       time.Duration
	now       func() time.Time
	observer  CacheObserver
	logger    *slog.Logger
}

func NewService(st store.Store, c cache.Cache, opts Options) *Service {
	if opts.CacheTTL <= 0 {
		opts.CacheTTL = DefaultCacheTTL
	}
	if opts.Clock == nil {
		opts.Clock = time.Now
	}
	if opts.Observer == nil {
		opts.Observer = noopObserver{}
	}
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	return &Service{
		store:     st,
		cache:     c,
		validator: opts.Validator,
		ttl:       opts.CacheTTL,
		now:       opts.Clock,
		observer:  opts.Observer,
		logger:    opts.Logger,
	}
}

type CreateRequest struct {
	URL       string
	Alias     string
	ExpiresAt *time.Time
}

func (s *Service) Create(ctx context.Context, req CreateRequest) (store.Link, error) {
	target, err := s.validator.Validate(req.URL)
	if err != nil {
		return store.Link{}, err
	}
	now := s.now().UTC()
	if req.ExpiresAt != nil && !req.ExpiresAt.After(now) {
		return store.Link{}, ErrInvalidExpiry
	}
	var expires *time.Time
	if req.ExpiresAt != nil {
		t := req.ExpiresAt.UTC()
		expires = &t
	}
	link := store.Link{URL: target, CreatedAt: now, ExpiresAt: expires}

	if req.Alias != "" {
		if err := ValidateAlias(req.Alias); err != nil {
			return store.Link{}, err
		}
		id, err := s.store.NextID(ctx)
		if err != nil {
			return store.Link{}, fmt.Errorf("allocate id: %w", err)
		}
		link.ID, link.Code, link.Custom = id, req.Alias, true
		if err := s.store.CreateLink(ctx, link); err != nil {
			if errors.Is(err, store.ErrCodeTaken) {
				return store.Link{}, ErrAliasTaken
			}
			return store.Link{}, fmt.Errorf("create link: %w", err)
		}
		return link, nil
	}

	for attempt := 0; attempt < maxGenerateAttempts; attempt++ {
		id, err := s.store.NextID(ctx)
		if err != nil {
			return store.Link{}, fmt.Errorf("allocate id: %w", err)
		}
		link.ID, link.Code = id, GenerateCode(id)
		err = s.store.CreateLink(ctx, link)
		if err == nil {
			return link, nil
		}
		if !errors.Is(err, store.ErrCodeTaken) {
			return store.Link{}, fmt.Errorf("create link: %w", err)
		}

		s.logger.Info("generated code collided with alias, retrying", "code", link.Code)
	}
	return store.Link{}, errors.New("could not allocate a unique code")
}

func GenerateCode(id int64) string {
	return Encode(uint64(id) + idOffset)
}

func (s *Service) Resolve(ctx context.Context, code string) (store.Link, error) {
	key := cacheKey(code)
	if raw, err := s.cache.Get(ctx, key); err == nil {
		var link store.Link
		if jsonErr := json.Unmarshal(raw, &link); jsonErr == nil {
			s.observer.CacheHit()
			return s.checkExpiry(link)
		}
		s.logger.Warn("corrupt cache entry, evicting", "key", key)
		_ = s.cache.Delete(ctx, key)
	} else if !errors.Is(err, cache.ErrMiss) {
		s.logger.Warn("cache get failed, falling back to store", "error", err)
	}
	s.observer.CacheMiss()

	link, err := s.store.GetLink(ctx, code)
	if err != nil {
		return store.Link{}, err
	}
	if raw, err := json.Marshal(link); err == nil {
		if err := s.cache.Set(ctx, key, raw, s.ttl); err != nil {
			s.logger.Warn("cache set failed", "error", err)
		}
	}
	return s.checkExpiry(link)
}

func (s *Service) checkExpiry(link store.Link) (store.Link, error) {
	if link.Expired(s.now()) {
		return link, ErrExpired
	}
	return link, nil
}

func (s *Service) List(ctx context.Context) ([]store.Link, error) {
	return s.store.ListLinks(ctx)
}

func (s *Service) Delete(ctx context.Context, code string) error {
	if err := s.store.DeleteLink(ctx, code); err != nil {
		return err
	}
	if err := s.cache.Delete(ctx, cacheKey(code)); err != nil {
		s.logger.Warn("cache delete failed", "error", err)
	}
	return nil
}

func (s *Service) Stats(ctx context.Context, code string) (store.Stats, error) {
	return s.store.Stats(ctx, code)
}

func cacheKey(code string) string { return "link:" + code }
