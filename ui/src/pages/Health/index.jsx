import { BsHdd, BsTablet, BsDatabase, BsClock } from "react-icons/bs";
import useResource from "../../hooks/useResource";
import {
  Activity,
  Metric,
  PageHeading,
  ResourceError,
  bytes,
} from "../../components/DashboardParts";
export default function Health() {
  const { data, error, loading, refresh } = useResource("health");
  return (
    <main className="workspace-page">
      <PageHeading
        eyebrow="INSTANCE ADMINISTRATION"
        title="Know your cloud."
        description="A clear view of the server that keeps your notes close."
        refresh={refresh}
        loading={loading}
      />
      <ResourceError error={error} />
      {!data && loading && <p role="status">Checking your instance…</p>}
      {data && (
        <>
          <div className="compatibility-banner">
            <span className="status-dot" />
            <div>
              <strong>
                Firmware compatibility · through {data.compatibility.through}
              </strong>
              <p>{data.compatibility.detail}</p>
            </div>
            <a
              href={data.compatibility.source}
              target="_blank"
              rel="noreferrer"
            >
              Upstream notes ↗
            </a>
          </div>
          <div className="metric-grid">
            <Metric
              icon={BsDatabase}
              label="Library database"
              value={data.database}
              detail="SQLite connection check"
            />
            <Metric
              icon={BsHdd}
              label="Storage used"
              value={`${data.storage.complete ? "" : "≥ "}${bytes(data.storage.bytes)}`}
              detail={`${data.storage.files.toLocaleString()} files · ${data.storage.status}`}
            />
            <Metric
              icon={BsTablet}
              label="Connected clients"
              value={data.clients}
              detail="All accounts · WebSocket connections"
            />
            <Metric
              icon={BsClock}
              label="Observation window"
              value={`${Math.max(0, Math.floor((Date.now() - new Date(data.observedSince)) / 3600000))} hours`}
              detail="Since the notification hub started"
            />
          </div>
          <div className="dashboard-grid">
            <section className="panel">
              <h2>Server health</h2>
              <dl className="health-details">
                <div>
                  <dt>rmfakecloud version</dt>
                  <dd>{data.version}</dd>
                </div>
                <div>
                  <dt>Updates</dt>
                  <dd>
                    <a
                      href="https://github.com/ddvk/rmfakecloud/releases"
                      target="_blank"
                      rel="noreferrer"
                    >
                      Check upstream releases ↗
                    </a>
                    <small>No automatic external requests</small>
                  </dd>
                </div>
                <div>
                  <dt>WebSocket hub</dt>
                  <dd>
                    {data.clients > 0
                      ? "Clients connected"
                      : "No clients connected"}
                    <small>
                      Connection count does not test the reverse proxy or MQTT.
                    </small>
                  </dd>
                </div>
                <div>
                  <dt>TLS certificate</dt>
                  <dd>
                    {data.certificate.status}
                    {data.certificate.expires && (
                      <small>
                        Expires{" "}
                        {new Date(data.certificate.expires).toLocaleString()}
                      </small>
                    )}
                    <small>{data.certificate.detail}</small>
                  </dd>
                </div>
                <div>
                  <dt>Document storage</dt>
                  <dd>
                    {data.storage.status}
                    <small>
                      Logical file sizes in the data directory. Not filesystem
                      free space or a write test. A scan is limited to two
                      seconds.
                    </small>
                  </dd>
                </div>
              </dl>
            </section>
            <Activity
              activity={data.activity}
              observedSince={data.observedSince}
            />
          </div>
          <section className="panel backup-panel">
            <div className="panel-heading">
              <div>
                <h2>Backup state</h2>
                <p className="muted">
                  Configured schedules and latest recorded runs, by account
                </p>
              </div>
            </div>
            {!data.backupsAvailable ? (
              <p role="status">Backup state is unavailable.</p>
            ) : !data.backups.length ? (
              <p className="muted">No backup policies are available yet.</p>
            ) : (
              <div className="table-responsive">
                <table className="health-table">
                  <thead>
                    <tr>
                      <th>Account</th>
                      <th>Schedule</th>
                      <th>Latest run</th>
                      <th>Result</th>
                    </tr>
                  </thead>
                  <tbody>
                    {data.backups.map((b) => (
                      <tr key={b.user}>
                        <td>{b.user}</td>
                        <td>{b.enabled ? b.schedule : "Disabled"}</td>
                        <td>
                          {b.lastRun
                            ? new Date(b.lastRun.started).toLocaleString()
                            : "Never run"}
                        </td>
                        <td>
                          {b.lastRun ? (
                            <>
                              {b.lastRun.status} · {b.lastRun.completed}{" "}
                              documents
                              {b.lastRun.failures?.length > 0 && (
                                <details>
                                  <summary>
                                    {b.lastRun.failures.length} errors
                                  </summary>
                                  {b.lastRun.failures.map((f, i) => (
                                    <p key={i}>{f}</p>
                                  ))}
                                </details>
                              )}
                            </>
                          ) : (
                            "No recorded backup"
                          )}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </section>
          <p className="chart-note">
            Checked {new Date(data.checkedAt).toLocaleString()} · Administrator
            access only
          </p>
        </>
      )}
    </main>
  );
}
