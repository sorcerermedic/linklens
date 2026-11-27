import { useMemo, useState } from "react";
import type { Link } from "../types";
import { formatDate, formatNumber, splitUrl } from "../lib/format";
import { ChartIcon, SearchIcon, TrashIcon } from "./icons";

interface Props {
  links: Link[];
  selected: string | null;
  onSelect: (code: string) => void;
  onDelete: (code: string) => void;
}

export function LinkTable({ links, selected, onSelect, onDelete }: Props) {
  const [query, setQuery] = useState("");

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return links;
    return links.filter(
      (l) =>
        l.code.toLowerCase().includes(q) || l.url.toLowerCase().includes(q),
    );
  }, [links, query]);

  const maxClicks = Math.max(1, ...links.map((l) => l.clicks));

  return (
    <div className="card card-flush">
      <div className="card-head padded">
        <div>
          <h2 className="card-title">Your links</h2>
          <p className="card-desc">
            {links.length === 0
              ? "Nothing here yet."
              : `${links.length} links, newest first.`}
          </p>
        </div>
        {links.length > 0 && (
          <label className="search">
            <SearchIcon />
            <input
              type="search"
              placeholder="Filter links"
              aria-label="Filter links"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
            />
          </label>
        )}
      </div>

      {links.length === 0 ? (
        <div className="empty">
          <p className="empty-title">No links yet.</p>
          <p className="empty-desc">
            Create your first one with the form and it will show up here.
          </p>
        </div>
      ) : (
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>Short link</th>
                <th>Destination</th>
                <th>Created</th>
                <th>Status</th>
                <th className="num">Clicks</th>
                <th aria-label="Actions" />
              </tr>
            </thead>
            <tbody>
              {filtered.map((l) => {
                const { host, path } = splitUrl(l.url);
                return (
                  <tr
                    key={l.code}
                    className={l.code === selected ? "selected" : undefined}
                  >
                    <td>
                      <div className="code-cell">
                        <a
                          href={l.shortUrl}
                          target="_blank"
                          rel="noreferrer"
                          className="mono code-link"
                        >
                          /{l.code}
                        </a>
                        {l.custom && (
                          <span className="badge badge-accent">alias</span>
                        )}
                      </div>
                    </td>
                    <td className="dest" title={l.url}>
                      <span className="dest-host">{host}</span>
                      <span className="dest-path">{path}</span>
                    </td>
                    <td className="muted nowrap">{formatDate(l.createdAt)}</td>
                    <td>
                      {l.expired ? (
                        <span className="badge badge-warn">expired</span>
                      ) : (
                        <span className="badge badge-ok">active</span>
                      )}
                    </td>
                    <td className="num">
                      <div className="clicks-cell">
                        <span className="mini-track" aria-hidden="true">
                          <span
                            className="mini-bar"
                            style={{
                              width: `${(l.clicks / maxClicks) * 100}%`,
                            }}
                          />
                        </span>
                        <span className="mono">{formatNumber(l.clicks)}</span>
                      </div>
                    </td>
                    <td>
                      <div className="row-actions">
                        <button
                          type="button"
                          className="btn btn-outline btn-sm"
                          onClick={() => onSelect(l.code)}
                        >
                          <ChartIcon />
                          Stats
                        </button>
                        <button
                          type="button"
                          className="btn btn-ghost btn-icon btn-danger"
                          onClick={() => onDelete(l.code)}
                          aria-label={`Delete ${l.code}`}
                        >
                          <TrashIcon />
                        </button>
                      </div>
                    </td>
                  </tr>
                );
              })}
              {filtered.length === 0 && (
                <tr>
                  <td colSpan={6} className="empty-row">
                    No links match “{query}”.
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
