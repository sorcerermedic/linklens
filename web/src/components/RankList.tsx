import type { ReactNode } from "react";
import { formatNumber } from "../lib/format";

export interface RankItem {
  key: string;
  label: ReactNode;
  value: number;
}

interface Props {
  label: string;
  items: RankItem[];
  limit?: number;
  empty?: string;
}

export function RankList({
  label,
  items,
  limit = 6,
  empty = "Nothing to show yet.",
}: Props) {
  if (items.length === 0) {
    return <p className="empty-inline">{empty}</p>;
  }
  const shown = items.slice(0, limit);
  const max = Math.max(...shown.map((i) => i.value), 1);
  return (
    <ul className="rank-list" aria-label={label}>
      {shown.map((item) => (
        <li key={item.key}>
          <div className="rank-row">
            <span className="rank-label">{item.label}</span>
            <span className="rank-value">{formatNumber(item.value)}</span>
          </div>
          <div className="rank-track" aria-hidden="true">
            <div
              className="rank-bar"
              style={{ width: `${Math.max(4, (item.value / max) * 100)}%` }}
            />
          </div>
        </li>
      ))}
    </ul>
  );
}
