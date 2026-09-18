import { BsArrowClockwise } from "react-icons/bs";
export function bytes(value = 0) {
  const units = ["B", "KB", "MB", "GB", "TB"];
  let i = 0;
  while (value >= 1024 && i < 4) {
    value /= 1024;
    i++;
  }
  return `${value.toFixed(i ? 1 : 0)} ${units[i]}`;
}
export function PageHeading({
  eyebrow,
  title,
  description,
  loading,
  refresh,
  children,
}) {
  return (
    <header className="page-heading">
      <div>
        <p className="eyebrow">{eyebrow}</p>
        <h1>{title}</h1>
        <p className="muted">{description}</p>
      </div>
      <div className="heading-actions">
        {children}
        <button className="soft-button" onClick={refresh} disabled={loading}>
          <BsArrowClockwise /> {loading ? "Loading…" : "Refresh"}
        </button>
      </div>
    </header>
  );
}
export function Metric({ icon: Icon, label, value, detail }) {
  return (
    <article className="metric-card">
      <span className="metric-icon">
        <Icon />
      </span>
      <span className="muted">{label}</span>
      <strong>{value}</strong>
      <small className="muted">{detail}</small>
    </article>
  );
}
export function Activity({ activity = {}, observedSince }) {
  const days = Array.from({ length: 7 }, (_, i) => {
    const d = new Date();
    d.setUTCDate(d.getUTCDate() - 6 + i);
    const key = d.toISOString().slice(0, 10);
    return {
      key,
      label: d.toLocaleDateString(undefined, {
        weekday: "short",
        timeZone: "UTC",
      }),
      value: activity[key] || 0,
    };
  });
  const max = Math.max(1, ...days.map((d) => d.value));
  return (
    <section className="panel">
      <div className="panel-heading">
        <div>
          <h2>Sync activity</h2>
          <p className="muted">
            Observed sync notifications · last 7 days, UTC
          </p>
        </div>
        <span className="pill">
          {days.reduce((n, d) => n + d.value, 0)} events
        </span>
      </div>
      <div
        className="activity-chart"
        role="img"
        aria-label={days
          .map((d) => `${d.key}: ${d.value} sync notifications`)
          .join(", ")}
      >
        {days.map((d) => (
          <div className="chart-column" key={d.key}>
            <span>{d.value}</span>
            <div className="bar-track">
              <div
                className="chart-bar"
                style={{ height: `${(d.value / max) * 100}%` }}
              />
            </div>
            <small>{d.label}</small>
          </div>
        ))}
      </div>
      <p className="chart-note">
        Collected since {new Date(observedSince).toLocaleString()}. Resets when
        the server restarts. Notifications do not verify a completed file
        transfer.
      </p>
    </section>
  );
}
export function ResourceError({ error }) {
  return (
    error && (
      <div role="alert" className="error-banner">
        Could not refresh data: {error}. Previously loaded values may be stale.
      </div>
    )
  );
}
