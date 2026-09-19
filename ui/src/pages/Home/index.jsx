import { Link } from "react-router-dom";
import {
  BsFiles,
  BsFolder,
  BsHdd,
  BsTablet,
  BsArrowUpRight,
} from "react-icons/bs";
import useResource from "../../hooks/useResource";
import {
  Activity,
  Metric,
  PageHeading,
  ResourceError,
  bytes,
} from "../../components/DashboardParts";
const colors = ["#a47cda", "#829abb", "#69638f", "#baa8c9"];
export default function Home() {
  const { data, error, loading, refresh } = useResource("dashboard");
  let offset = 0;
  const segments = data
    ? Object.entries(data.types).map(([name, count], i) => {
        const start = offset;
        offset += data.documents ? (count / data.documents) * 100 : 0;
        return {
          name,
          count,
          color: colors[i],
          gradient: `${colors[i]} ${start}% ${offset}%`,
        };
      })
    : [];
  return (
    <main className="workspace-page">
      <PageHeading
        eyebrow="YOUR PERSONAL CLOUD"
        title="A little space for big ideas."
        description="Your documents, connected devices, and the activity between them."
        loading={loading}
        refresh={refresh}
      >
        <Link className="primary-button" to="/documents">
          Open library <BsArrowUpRight />
        </Link>
      </PageHeading>
      <ResourceError error={error} />
      {!data && loading && <p role="status">Loading your cloud…</p>}
      {data && (
        <>
          <div className="metric-grid">
            <Metric
              icon={BsFiles}
              label="Documents"
              value={data.documents.toLocaleString()}
              detail="In your active library"
            />
            <Metric
              icon={BsFolder}
              label="Folders"
              value={data.folders.toLocaleString()}
              detail="A place for every thought"
            />
            <Metric
              icon={BsHdd}
              label="Document size"
              value={bytes(data.documentBytes)}
              detail="Reported by document metadata"
            />
            <Metric
              icon={BsTablet}
              label="Connected clients"
              value={data.clients}
              detail="Your open WebSocket connections"
            />
          </div>
          <div className="dashboard-grid">
            <Activity
              activity={data.activity}
              observedSince={data.observedSince}
            />
            <section className="panel">
              <div className="panel-heading">
                <div>
                  <h2>Your library, at a glance</h2>
                  <p className="muted">Document formats</p>
                </div>
              </div>
              <div className="donut-wrap">
                <div
                  className="donut"
                  role="img"
                  aria-label={segments
                    .map((s) => `${s.name}: ${s.count}`)
                    .join(", ")}
                  style={{
                    background: data.documents
                      ? `conic-gradient(${segments.map((s) => s.gradient).join(",")})`
                      : "var(--border)",
                  }}
                >
                  <div>
                    <strong>{data.documents}</strong>
                    <span>documents</span>
                  </div>
                </div>
              </div>
              <ul className="chart-legend">
                {segments.map((s) => (
                  <li key={s.name}>
                    <span>
                      <i style={{ background: s.color }} />
                      {s.name}
                    </span>
                    <strong>{s.count}</strong>
                  </li>
                ))}
              </ul>
              {data.documents === 0 && (
                <p className="muted">
                  Connect your tablet or upload a document to get started.
                </p>
              )}
            </section>
          </div>
          <div className="quick-links">
            <Link to="/connect">
              <BsTablet />
              <div>
                <strong>Make the connection</strong>
                <span>Pair a tablet or desktop app</span>
              </div>
              <BsArrowUpRight />
            </Link>
            <Link to="/integrations">
              <BsFolder />
              <div>
                <strong>Bring your storage along</strong>
                <span>Manage your connected services</span>
              </div>
              <BsArrowUpRight />
            </Link>
          </div>
          <p className="chart-note">
            Last refreshed {new Date(data.checkedAt).toLocaleTimeString()} ·
            Your account only
          </p>
        </>
      )}
    </main>
  );
}
