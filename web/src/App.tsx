import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { api as defaultApi, type Api } from "./api";
import { CreateLinkForm } from "./components/CreateLinkForm";
import { LinkTable } from "./components/LinkTable";
import { RankList } from "./components/RankList";
import { StatCard } from "./components/StatCard";
import { StatsPanel } from "./components/StatsPanel";
import {
  CursorIcon,
  LinkIcon,
  PulseIcon,
  RefreshIcon,
  TrophyIcon,
} from "./components/icons";
import { formatNumber, percent } from "./lib/format";
import type { CreateLinkInput, Link, Stats } from "./types";

interface Props {
  api?: Api;
}

const HASH_PREFIX = "#/stats/";

function codeFromHash(): string | null {
  const h = window.location.hash;
  return h.startsWith(HASH_PREFIX)
    ? decodeURIComponent(h.slice(HASH_PREFIX.length))
    : null;
}

function setHash(code: string | null) {
  const next = code ? `${HASH_PREFIX}${encodeURIComponent(code)}` : "";
  if (window.location.hash === next) return;
  window.history.replaceState(
    null,
    "",
    `${window.location.pathname}${window.location.search}${next}`,
  );
}

export function App({ api = defaultApi }: Props) {
  const [links, setLinks] = useState<Link[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [selected, setSelected] = useState<string | null>(null);
  const [stats, setStats] = useState<Stats | null>(null);
  const restored = useRef(false);

  const refresh = useCallback(async () => {
    try {
      setLinks(await api.listLinks());
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setLoading(false);
    }
  }, [api]);

  const handleSelect = useCallback(
    async (code: string) => {
      setSelected(code);
      setStats(null);
      setHash(code);
      try {
        setStats(await api.getStats(code));
      } catch (err) {
        setError(err instanceof Error ? err.message : String(err));
      }
    },
    [api],
  );

  const closeStats = useCallback(() => {
    setSelected(null);
    setStats(null);
    setHash(null);
  }, []);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  useEffect(() => {
    if (loading || restored.current) return;
    restored.current = true;
    const code = codeFromHash();
    if (code && links.some((l) => l.code === code)) void handleSelect(code);
  }, [loading, links, handleSelect]);

  async function handleCreate(input: CreateLinkInput) {
    const link = await api.createLink(input);
    await refresh();
    return link;
  }

  async function handleDelete(code: string) {
    if (!window.confirm(`Delete /${code} and all of its click data?`)) return;
    try {
      await api.deleteLink(code);
      if (selected === code) closeStats();
      await refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    }
  }

  const summary = useMemo(() => {
    const totalClicks = links.reduce((sum, l) => sum + l.clicks, 0);
    const active = links.filter((l) => !l.expired).length;
    const custom = links.filter((l) => l.custom).length;
    const ranked = [...links].sort(
      (a, b) => b.clicks - a.clicks || a.code.localeCompare(b.code),
    );
    return { totalClicks, active, custom, ranked, top: ranked[0] };
  }, [links]);

  const selectedLink = links.find((l) => l.code === selected);

  return (
    <div className="shell">
      <header className="topbar">
        <div className="topbar-inner">
          <div className="brand">
            <span className="brand-mark" aria-hidden="true">
              <LinkIcon />
            </span>
            <span className="brand-name">linklens</span>
            <span className="brand-tag">URL shortener · click analytics</span>
          </div>
          <button
            type="button"
            className="btn btn-outline btn-sm"
            onClick={() => void refresh()}
          >
            <RefreshIcon />
            Refresh
          </button>
        </div>
      </header>

      <main className="page">
        <div className="page-head">
          <div>
            <h1 className="page-title">Overview</h1>
            <p className="page-desc">
              Every redirect is counted asynchronously, so short links stay
              fast.
            </p>
          </div>
        </div>

        {error && (
          <p role="alert" className="banner-error">
            {error}
          </p>
        )}

        <section className="stat-grid" aria-label="Summary">
          <StatCard
            label="Total links"
            value={formatNumber(links.length)}
            hint={`${summary.custom} with a custom alias`}
            icon={<LinkIcon />}
          />
          <StatCard
            label="Total clicks"
            value={formatNumber(summary.totalClicks)}
            hint={
              links.length
                ? `${formatNumber(Math.round(summary.totalClicks / links.length))} per link on average`
                : "no traffic yet"
            }
            icon={<CursorIcon />}
          />
          <StatCard
            label="Active links"
            value={formatNumber(summary.active)}
            hint={`${links.length - summary.active} expired (410 Gone)`}
            icon={<PulseIcon />}
          />
          <StatCard
            label="Top link"
            value={
              summary.top && summary.top.clicks > 0 ? (
                <span className="mono">/{summary.top.code}</span>
              ) : (
                "–"
              )
            }
            hint={
              summary.top && summary.top.clicks > 0
                ? `${percent(summary.top.clicks, summary.totalClicks)} of all clicks`
                : "waiting for the first click"
            }
            icon={<TrophyIcon />}
          />
        </section>

        <div className="layout">
          <aside className="side">
            <CreateLinkForm onCreate={handleCreate} />
            <div className="card">
              <div className="card-head">
                <div>
                  <h2 className="card-title">Top performers</h2>
                  <p className="card-desc">Clicks per link, all time.</p>
                </div>
              </div>
              <RankList
                label="Top performers"
                limit={5}
                empty="No clicks yet."
                items={summary.ranked
                  .filter((l) => l.clicks > 0)
                  .map((l) => ({
                    key: l.code,
                    value: l.clicks,
                    label: <span className="mono">{l.code}</span>,
                  }))}
              />
            </div>
          </aside>

          <section className="main-col" aria-label="Links">
            {loading ? (
              <div className="card skeleton" aria-busy="true">
                <p className="muted">Loading…</p>
              </div>
            ) : (
              <LinkTable
                links={links}
                selected={selected}
                onSelect={(c) => void handleSelect(c)}
                onDelete={(c) => void handleDelete(c)}
              />
            )}
          </section>
        </div>
      </main>

      {selected && stats && (
        <StatsPanel
          code={selected}
          stats={stats}
          link={selectedLink}
          onClose={closeStats}
        />
      )}
    </div>
  );
}
