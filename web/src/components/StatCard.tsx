import type { ReactNode } from "react";

interface Props {
  label: string;
  value: ReactNode;
  hint?: ReactNode;
  icon?: ReactNode;
  testId?: string;
}

export function StatCard({ label, value, hint, icon, testId }: Props) {
  return (
    <div className="stat-card">
      <span className="stat-stripe" aria-hidden="true" />
      <div className="stat-top">
        <div className="stat-text">
          <p className="eyebrow">{label}</p>
          <p className="stat-value" data-testid={testId}>
            {value}
          </p>
        </div>
        {icon && <div className="stat-icon">{icon}</div>}
      </div>
      {hint && <p className="stat-hint">{hint}</p>}
    </div>
  );
}
