package postgres

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"sort"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/portfolio-author/linklens/internal/store"
)

//go:embed migrations/*.sql
var migrations embed.FS

const uniqueViolation = "23505"

type Store struct {
	pool *pgxpool.Pool
}

func Open(ctx context.Context, dsn string) (*Store, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}
	return &Store{pool: pool}, nil
}

func (s *Store) Migrate(ctx context.Context) error {
	entries, err := fs.ReadDir(migrations, "migrations")
	if err != nil {
		return err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	for _, name := range names {
		sql, err := migrations.ReadFile("migrations/" + name)
		if err != nil {
			return err
		}
		if _, err := s.pool.Exec(ctx, string(sql)); err != nil {
			return fmt.Errorf("migration %s: %w", name, err)
		}
	}
	return nil
}

func (s *Store) NextID(ctx context.Context) (int64, error) {
	var id int64
	err := s.pool.QueryRow(ctx, `SELECT nextval('links_id_seq')`).Scan(&id)
	return id, err
}

func (s *Store) CreateLink(ctx context.Context, l store.Link) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO links (id, code, url, custom, created_at, expires_at) VALUES ($1, $2, $3, $4, $5, $6)`,
		l.ID, l.Code, l.URL, l.Custom, l.CreatedAt, l.ExpiresAt)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == uniqueViolation {
		return store.ErrCodeTaken
	}
	return err
}

func (s *Store) GetLink(ctx context.Context, code string) (store.Link, error) {
	var l store.Link
	err := s.pool.QueryRow(ctx,
		`SELECT id, code, url, custom, created_at, expires_at FROM links WHERE code = $1`, code).
		Scan(&l.ID, &l.Code, &l.URL, &l.Custom, &l.CreatedAt, &l.ExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return store.Link{}, store.ErrNotFound
	}
	return l, err
}

func (s *Store) ListLinks(ctx context.Context) ([]store.Link, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT l.id, l.code, l.url, l.custom, l.created_at, l.expires_at, count(c.id)
		FROM links l
		LEFT JOIN clicks c ON c.link_id = l.id
		GROUP BY l.id
		ORDER BY l.created_at DESC, l.id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []store.Link{}
	for rows.Next() {
		var l store.Link
		if err := rows.Scan(&l.ID, &l.Code, &l.URL, &l.Custom, &l.CreatedAt, &l.ExpiresAt, &l.Clicks); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

func (s *Store) DeleteLink(ctx context.Context, code string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM links WHERE code = $1`, code)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return store.ErrNotFound
	}
	return nil
}

func (s *Store) RecordClicks(ctx context.Context, clicks []store.Click) error {
	if len(clicks) == 0 {
		return nil
	}
	batch := &pgx.Batch{}
	for _, c := range clicks {
		batch.Queue(`
			INSERT INTO clicks (link_id, clicked_at, country, device, referrer)
			SELECT id, $2, $3, $4, $5 FROM links WHERE code = $1`,
			c.Code, c.At, c.Country, c.Device, c.Referrer)
	}
	results := s.pool.SendBatch(ctx, batch)
	defer results.Close()
	for range clicks {
		if _, err := results.Exec(); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) Stats(ctx context.Context, code string) (store.Stats, error) {
	var linkID int64
	err := s.pool.QueryRow(ctx, `SELECT id FROM links WHERE code = $1`, code).Scan(&linkID)
	if errors.Is(err, pgx.ErrNoRows) {
		return store.Stats{}, store.ErrNotFound
	}
	if err != nil {
		return store.Stats{}, err
	}

	st := store.Stats{
		ByDay:        []store.DayCount{},
		ByDevice:     map[string]int64{},
		ByCountry:    map[string]int64{},
		TopReferrers: []store.ReferrerCount{},
	}

	batch := &pgx.Batch{}
	batch.Queue(`SELECT count(*) FROM clicks WHERE link_id = $1`, linkID)
	batch.Queue(`SELECT to_char(clicked_at AT TIME ZONE 'UTC', 'YYYY-MM-DD') AS day, count(*)
	             FROM clicks WHERE link_id = $1 GROUP BY day ORDER BY day`, linkID)
	batch.Queue(`SELECT device, count(*) FROM clicks WHERE link_id = $1 GROUP BY device`, linkID)
	batch.Queue(`SELECT country, count(*) FROM clicks WHERE link_id = $1 GROUP BY country`, linkID)
	batch.Queue(`SELECT referrer, count(*) AS n FROM clicks WHERE link_id = $1
	             GROUP BY referrer ORDER BY n DESC, referrer LIMIT $2`, linkID, store.TopReferrersLimit)
	results := s.pool.SendBatch(ctx, batch)
	defer results.Close()

	if err := results.QueryRow().Scan(&st.TotalClicks); err != nil {
		return store.Stats{}, err
	}
	if err := scanPairs(results, func(k string, n int64) {
		st.ByDay = append(st.ByDay, store.DayCount{Day: k, Clicks: n})
	}); err != nil {
		return store.Stats{}, err
	}
	if err := scanPairs(results, func(k string, n int64) { st.ByDevice[k] = n }); err != nil {
		return store.Stats{}, err
	}
	if err := scanPairs(results, func(k string, n int64) { st.ByCountry[k] = n }); err != nil {
		return store.Stats{}, err
	}
	if err := scanPairs(results, func(k string, n int64) {
		st.TopReferrers = append(st.TopReferrers, store.ReferrerCount{Referrer: k, Clicks: n})
	}); err != nil {
		return store.Stats{}, err
	}
	return st, nil
}

func scanPairs(results pgx.BatchResults, visit func(key string, n int64)) error {
	rows, err := results.Query()
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var k string
		var n int64
		if err := rows.Scan(&k, &n); err != nil {
			return err
		}
		visit(k, n)
	}
	return rows.Err()
}

func (s *Store) Ping(ctx context.Context) error { return s.pool.Ping(ctx) }

func (s *Store) Close() { s.pool.Close() }
