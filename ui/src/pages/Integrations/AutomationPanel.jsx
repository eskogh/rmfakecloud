import React, { useEffect, useState } from "react";
import { Alert, Button, Card, Form, Modal, Table } from "react-bootstrap";
import { Link } from "react-router-dom";
import RulesPanel from "./RulesPanel";
import api from "../../services/api.service";

const empty = () => ({ name: "", type: "webhook", enabled: true, user_id: "", chat_id: "", endpoint: "", events: ["document.updated"], headers: {}, auth: { type: "", token: "" }, signing: { enabled: false, secret: "" }, timeout: "10s", retry: { count: 0 }, filters: [] });

export default function AutomationPanel() {
  const user = JSON.parse(localStorage.getItem("currentUser") || "null");
  if (!user?.Roles?.includes("Admin")) return null;
  return <AutomationAdmin />;
}
function AutomationAdmin() {
  const [items, setItems] = useState([]);
  const [editing, setEditing] = useState(null);
  const [headers, setHeaders] = useState("{}");
  const [filters, setFilters] = useState("[]");
  const [history, setHistory] = useState(null);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [busy, setBusy] = useState(false);
  async function refresh() { setItems(await api.automation()); }
  useEffect(() => {
    refresh().catch(e => setError(e.message));
    const timer = setInterval(() => refresh().catch(() => {}), 5000);
    return () => clearInterval(timer);
  }, []);
  function edit(item) {
    setError(""); setEditing(item); setHeaders(JSON.stringify(item.headers || {}, null, 2)); setFilters(JSON.stringify(item.filters || [], null, 2));
  }
  async function act(fn) {
    setBusy(true); setError("");
    try { await fn(); await refresh(); } catch (e) { setError(e.message); } finally { setBusy(false); }
  }
  function change(key, value) { setEditing(current => ({ ...current, [key]: value })); }
  async function save(e) {
    e.preventDefault();
    await act(async () => {
      const config = { ...editing, headers: editing.type === "webhook" ? JSON.parse(headers) : {}, filters: JSON.parse(filters) };
      await api.automation(config.id ? `/${config.id}` : "", config.id ? "PUT" : "POST", config);
      setEditing(null); setNotice("Integration saved.");
    });
  }
  return <section className="mb-4">
    <div className="d-flex justify-content-between align-items-center mb-2"><h4>Automations</h4><Button onClick={() => edit(empty())}>Add integration</Button></div>
    <p>Send document and sync events to your services. For workflow tools such as n8n or Home Assistant, choose Webhook.</p>
    <p><Link to="/settings/mail">Configure email</Link></p>
    {error && <Alert variant="danger">{error}</Alert>}
    {notice && <Alert variant="info" dismissible onClose={() => setNotice("")}>{notice}</Alert>}
    <Card className="mb-3"><Table responsive className="mb-0"><thead><tr><th>Name / type</th><th>Status</th><th>Events</th><th>Last delivery</th><th>Actions</th></tr></thead><tbody>
      {!items.length && <tr><td colSpan={5}>No automations configured.</td></tr>}
      {items.map(item => <tr key={item.id}>
        <td>{item.name}<br /><small>{item.type}</small></td>
        <td>{item.enabled ? "Enabled" : "Disabled"}</td>
        <td>{item.events?.join(", ")}</td>
        <td>{item.last_delivery ? <>{new Date(item.last_delivery.timestamp).toLocaleString()}<br />{item.last_delivery.success ? "Succeeded" : item.last_delivery.error}</> : "No deliveries"}</td>
        <td className="d-flex flex-wrap gap-1">
          <Button size="sm" disabled={busy} onClick={() => edit(item)}>Configure</Button>
          <Button size="sm" variant="outline-secondary" disabled={busy} onClick={() => act(() => api.automation(`/${item.id}`, "PUT", { ...item, enabled: !item.enabled }))}>{item.enabled ? "Disable" : "Enable"}</Button>
          <Button size="sm" disabled={busy} onClick={() => act(async () => { await api.automation(`/${item.id}/test`, "POST"); setNotice("Test queued. Delivery status refreshes automatically."); })}>Test</Button>
          <Button size="sm" variant="outline-secondary" onClick={() => act(async () => setHistory({ id: item.id, name: item.name, entries: await api.automation(`/${item.id}/deliveries`) }))}>History</Button>
          <Button size="sm" variant="outline-danger" disabled={busy} onClick={() => { if (window.confirm(`Delete ${item.name}?`)) act(() => api.automation(`/${item.id}`, "DELETE")); }}>Delete</Button>
        </td>
      </tr>)}
    </tbody></Table></Card>
    <RulesPanel integrations={items} />
    <Modal show={!!editing} onHide={() => setEditing(null)} size="lg"><Modal.Header closeButton><Modal.Title>{editing?.id ? "Configure integration" : "Add integration"}</Modal.Title></Modal.Header>
      {editing && <Form onSubmit={save}><Modal.Body>
        {error && <Alert variant="danger">{error}</Alert>}
        <Form.Group className="mb-3"><Form.Label>Name</Form.Label><Form.Control required value={editing.name} onChange={e => change("name", e.target.value)} /></Form.Group>
        <Form.Group className="mb-3"><Form.Label>Type</Form.Label><Form.Select value={editing.type} disabled={!!editing.id} onChange={e => setEditing(current => ({ ...current, type: e.target.value, endpoint: "", headers: {}, auth: { type: "", token: "" }, signing: { enabled: false, secret: "" } }))}><option value="webhook">Webhook / Advanced automation</option><option value="telegram">Telegram</option><option value="discord">Discord</option></Form.Select></Form.Group>
        <Form.Group className="mb-3"><Form.Label>User ID</Form.Label><Form.Control disabled={!!editing.id} value={editing.user_id} placeholder="Your user (default)" onChange={e => change("user_id", e.target.value)} /><Form.Text>Only events belonging to this user are delivered.</Form.Text></Form.Group>
        {editing.type !== "telegram" && <Form.Group className="mb-3"><Form.Label>Endpoint URL</Form.Label><Form.Control type="url" required={!editing.endpointConfigured} value={editing.endpoint} placeholder={editing.endpointConfigured ? "Saved — leave blank to keep" : "https://example.com/webhook"} onChange={e => change("endpoint", e.target.value)} /></Form.Group>}
        {editing.type === "telegram" && <><Form.Group className="mb-3"><Form.Label>Bot token</Form.Label><Form.Control required={!editing.tokenConfigured} type="password" autoComplete="new-password" value={editing.auth.token || ""} placeholder={editing.tokenConfigured ? "Saved — leave blank to keep" : "Token or ${ENV_VARIABLE}"} onChange={e => change("auth", { type: "", token: e.target.value })} /></Form.Group><Form.Group className="mb-3"><Form.Label>Chat ID</Form.Label><Form.Control required value={editing.chat_id || ""} onChange={e => change("chat_id", e.target.value)} /></Form.Group></>}
        <Form.Group className="mb-3"><Form.Label>Events (comma separated)</Form.Label><Form.Control value={editing.events?.join(", ") || ""} onChange={e => change("events", e.target.value === "" ? [] : e.target.value.split(",").map(x => x.trim()))} /><Form.Text>For example document.created, document.updated, document.deleted, sync.*</Form.Text></Form.Group>
        <Form.Group className="mb-3"><Form.Label>Timeout</Form.Label><Form.Control value={editing.timeout} placeholder="10s" onChange={e => change("timeout", e.target.value)} /></Form.Group>
        <Form.Group className="mb-3"><Form.Label>Retries</Form.Label><Form.Control type="number" min="0" max="3" value={editing.retry.count} onChange={e => change("retry", { count: Number(e.target.value) })} /><Form.Text>Retries may deliver an event more than once. Webhook receivers should deduplicate delivery IDs. Native services may receive duplicate messages.</Form.Text></Form.Group>
        <Form.Check className="mb-3" label="Enabled" checked={editing.enabled} onChange={e => change("enabled", e.target.checked)} />
        {editing.type === "webhook" && <><Form.Check label="Bearer authentication" checked={editing.auth.type === "bearer"} onChange={e => change("auth", { ...editing.auth, type: e.target.checked ? "bearer" : "" })} />
        {editing.auth.type === "bearer" && <Form.Control aria-label="Bearer token" className="mb-3" type="password" autoComplete="new-password" value={editing.auth.token || ""} placeholder={editing.tokenConfigured ? "Token saved — leave blank to keep" : "Token or ${ENV_VARIABLE}"} onChange={e => change("auth", { ...editing.auth, token: e.target.value })} />}
        <Form.Check label="Sign deliveries" checked={editing.signing.enabled} onChange={e => change("signing", { ...editing.signing, enabled: e.target.checked })} />
        {editing.signing.enabled && <Form.Control aria-label="Signing secret" className="mb-3" type="password" autoComplete="new-password" value={editing.signing.secret || ""} placeholder={editing.signingConfigured ? "Secret saved — leave blank to keep" : "Shared secret or ${ENV_VARIABLE}"} onChange={e => change("signing", { ...editing.signing, secret: e.target.value })} />}
        <Form.Group className="mb-3"><Form.Label>Custom headers (JSON object)</Form.Label><Form.Control as="textarea" rows={3} value={headers} onChange={e => setHeaders(e.target.value)} /><Form.Text>Saved values are hidden. Blank values keep existing secrets; remove a key to delete a header.</Form.Text></Form.Group></>}
        <Form.Group><Form.Label>Filters (JSON array)</Form.Label><Form.Control as="textarea" rows={3} value={filters} onChange={e => setFilters(e.target.value)} /><Form.Text>{'Example: [{"field":"document.name","operator":"starts_with","value":"Work"}]'}</Form.Text></Form.Group>
      </Modal.Body><Modal.Footer><Button variant="secondary" onClick={() => setEditing(null)}>Cancel</Button><Button type="submit" disabled={busy}>Save</Button></Modal.Footer></Form>}
    </Modal>
    <Modal show={!!history} onHide={() => setHistory(null)} size="lg"><Modal.Header closeButton><Modal.Title>{history?.name} delivery history</Modal.Title></Modal.Header><Modal.Body>
      <Button size="sm" className="mb-2" onClick={() => act(async () => setHistory({ ...history, entries: await api.automation(`/${history.id}/deliveries`) }))}>Refresh</Button>
      <Table responsive><thead><tr><th>Time / delivery ID</th><th>Event</th><th>Result</th><th>Attempts / duration</th></tr></thead><tbody>
        {history?.entries.slice().reverse().map(d => <tr key={d.id}><td>{new Date(d.timestamp).toLocaleString()}<br /><small>{d.id}</small></td><td>{d.event}</td><td>{d.success ? "Success" : d.error} {d.http_status || ""}</td><td>{d.attempts} / {d.duration_ms} ms</td></tr>)}
      </tbody></Table>
    </Modal.Body></Modal>
  </section>;
}
