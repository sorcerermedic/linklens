import { useEffect } from "react";
import {
  Area,
  AreaChart,
  CartesianGrid,
  Cell,
  Pie,
  PieChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import { toSortedEntries } from "../lib/entries";
import {
  countryName,
  formatDay,
  formatNumber,
  percent,
  splitUrl,
} from "../lib/format";
import type { Link, Stats } from "../types";
import { RankList } from "./RankList";
import { CloseIcon, ExternalIcon } from "./icons";

const DEVICE_COLORS = [
  "var(--chart-1)",
  "var(--chart-2)",
  "var(--chart-3)",
  "var(--chart-4)",
  "var(--chart-5)",
];

const tooltipStyle = {
  background: "var(--surface)",
  border: "1px solid var(--border)",
  borderRadius: 10,
  boxShadow: "var(--shadow-md)",
  fontSize: 12,
  color: "var(--text)",
};

interface Props {
  code: string;
  stats: Stats;
  link?: Link;
  onClose: () => void;
}

export function StatsPanel({ code, stats, link, onClose }: Props) {
  const devices = toSortedEntries(stats.byDevice);
  const countries = toSortedEntries(stats.byCountry);
  const peak = stats.byDay.reduce<{ day: string; clicks: number } | null>(
    (best, d) => (best === null || d.clicks > best.clicks ? d : best),
    null,
  );
  const mobile = (stats.byDevice.mobile ?? 0) + (stats.byDevice.tablet ?? 0);
  const topCountry = countries[0];
  const chartData = stats.byDay.map((d) => ({ ...d, label: formatDay(d.day) }));
  const target = link ? splitUrl(link.url) : null;

  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if (e.key === "Escape") onClose();
    }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onClose]);

  return (
    <div className="sheet-root">
      <div className="sheet-overlay" onClick={onClose} aria-hidden="true" />
      <section className="sheet" aria-label={`Statistics for ${code}`}>
        <header className="sheet-head">
          <div className="sheet-title-wrap">
            <p className="eyebrow">Link analytics</p>
            <h2 className="sheet-title mono">/{code}</h2>
            {link && target && (
              <a
                className="sheet-target"
                href={link.url}
                target="_blank"
                rel="noreferrer"
              >
                {target.host}
                {target.path}
                <ExternalIcon />
              </a>
            )}
          </div>
          <button
            type="button"
            className="btn btn-ghost btn-icon"
            onClick={onClose}
            aria-label="Close"
          >
            <CloseIcon />
          </button>
        </header>

        <div className="sheet-body">
          <div className="mini-stats">
            <div className="mini-stat">
              <p className="eyebrow">Total clicks</p>
              <p className="mini-value" data-testid="total-clicks">
                {formatNumber(stats.totalClicks)}
              </p>
              <p className="mini-hint">all time</p>
            </div>
            <div className="mini-stat">
              <p className="eyebrow">Peak day</p>
              <p className="mini-value">
                {peak ? formatNumber(peak.clicks) : "–"}
              </p>
              <p className="mini-hint">
                {peak ? formatDay(peak.day) : "no data"}
              </p>
            </div>
            <div className="mini-stat">
              <p className="eyebrow">Top country</p>
              <p className="mini-value">
                {topCountry ? countryName(topCountry.name) : "–"}
              </p>
              <p className="mini-hint">
                {topCountry
                  ? `${percent(topCountry.value, stats.totalClicks)} of clicks`
                  : "no data"}
              </p>
            </div>
            <div className="mini-stat">
              <p className="eyebrow">Mobile share</p>
              <p className="mini-value">{percent(mobile, stats.totalClicks)}</p>
              <p className="mini-hint">phones + tablets</p>
            </div>
          </div>

          {stats.totalClicks === 0 ? (
            <div className="empty">
              <p className="empty-title">No clicks recorded yet.</p>
              <p className="empty-desc">
                Open the short link to generate some traffic, then refresh.
              </p>
            </div>
          ) : (
            <>
              <div className="panel">
                <div className="panel-head">
                  <h3 className="panel-title">Clicks by day</h3>
                  <span className="panel-note">UTC</span>
                </div>
                <div className="chart-box">
                  <ResponsiveContainer width="100%" height={220}>
                    <AreaChart
                      data={chartData}
                      margin={{ top: 8, right: 8, left: -18, bottom: 0 }}
                    >
                      <defs>
                        <linearGradient
                          id="clicksFill"
                          x1="0"
                          y1="0"
                          x2="0"
                          y2="1"
                        >
                          <stop
                            offset="0%"
                            stopColor="var(--chart-1)"
                            stopOpacity={0.35}
                          />
                          <stop
                            offset="100%"
                            stopColor="var(--chart-1)"
                            stopOpacity={0}
                          />
                        </linearGradient>
                      </defs>
                      <CartesianGrid
                        stroke="var(--border)"
                        strokeDasharray="3 3"
                        vertical={false}
                      />
                      <XAxis
                        dataKey="label"
                        tickLine={false}
                        axisLine={false}
                        tick={{ fill: "var(--muted)", fontSize: 11 }}
                        minTickGap={12}
                      />
                      <YAxis
                        allowDecimals={false}
                        tickLine={false}
                        axisLine={false}
                        tick={{ fill: "var(--muted)", fontSize: 11 }}
                      />
                      <Tooltip
                        contentStyle={tooltipStyle}
                        cursor={{ stroke: "var(--border-strong)" }}
                      />
                      <Area
                        type="monotone"
                        dataKey="clicks"
                        stroke="var(--chart-1)"
                        strokeWidth={2}
                        fill="url(#clicksFill)"
                        dot={false}
                        activeDot={{ r: 4 }}
                        isAnimationActive={false}
                      />
                    </AreaChart>
                  </ResponsiveContainer>
                </div>
              </div>

              <div className="panel-grid">
                <div className="panel">
                  <div className="panel-head">
                    <h3 className="panel-title">Devices</h3>
                  </div>
                  <div className="donut-wrap">
                    <div className="donut">
                      <ResponsiveContainer width="100%" height={140}>
                        <PieChart>
                          <Pie
                            data={devices}
                            dataKey="value"
                            nameKey="name"
                            innerRadius={44}
                            outerRadius={64}
                            paddingAngle={2}
                            stroke="none"
                            isAnimationActive={false}
                          >
                            {devices.map((d, i) => (
                              <Cell
                                key={d.name}
                                fill={DEVICE_COLORS[i % DEVICE_COLORS.length]}
                              />
                            ))}
                          </Pie>
                          <Tooltip contentStyle={tooltipStyle} />
                        </PieChart>
                      </ResponsiveContainer>
                    </div>
                    <ul className="legend" aria-label="Devices">
                      {devices.map((d, i) => (
                        <li key={d.name}>
                          <span
                            className="dot"
                            style={{
                              background:
                                DEVICE_COLORS[i % DEVICE_COLORS.length],
                            }}
                          />
                          <span className="legend-name">{d.name}</span>
                          <span className="legend-value">
                            {percent(d.value, stats.totalClicks)}
                          </span>
                        </li>
                      ))}
                    </ul>
                  </div>
                </div>

                <div className="panel">
                  <div className="panel-head">
                    <h3 className="panel-title">Countries</h3>
                  </div>
                  <RankList
                    label="Countries"
                    limit={5}
                    items={countries.map((c) => ({
                      key: c.name,
                      value: c.value,
                      label: (
                        <>
                          <span className="chip">{c.name}</span>
                          {countryName(c.name)}
                        </>
                      ),
                    }))}
                  />
                </div>
              </div>

              <div className="panel">
                <div className="panel-head">
                  <h3 className="panel-title">Top referrers</h3>
                </div>
                <RankList
                  label="Top referrers"
                  limit={6}
                  items={stats.topReferrers.map((r) => ({
                    key: r.referrer,
                    value: r.clicks,
                    label: r.referrer,
                  }))}
                />
              </div>
            </>
          )}
        </div>
      </section>
    </div>
  );
}
