CREATE SEQUENCE IF NOT EXISTS links_id_seq;

CREATE TABLE IF NOT EXISTS links (
    id          BIGINT PRIMARY KEY,
    code        TEXT NOT NULL UNIQUE,
    url         TEXT NOT NULL,
    custom      BOOLEAN NOT NULL DEFAULT FALSE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at  TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS clicks (
    id          BIGSERIAL PRIMARY KEY,
    link_id     BIGINT NOT NULL REFERENCES links(id) ON DELETE CASCADE,
    clicked_at  TIMESTAMPTZ NOT NULL,
    country     TEXT NOT NULL DEFAULT '',
    device      TEXT NOT NULL DEFAULT '',
    referrer    TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS clicks_link_id_clicked_at_idx ON clicks (link_id, clicked_at);
