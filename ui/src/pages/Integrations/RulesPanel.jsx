import React, { useEffect, useState } from "react";
import { Alert, Button, Card, Form } from "react-bootstrap";
import api from "../../services/api.service";

export default function RulesPanel({ integrations }) {
  const [rules, setRules] = useState([]);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [busy, setBusy] = useState(false);
  useEffect(() => { api.automation("/rules").then(setRules).catch(e => setError(e.message)); }, []);
  function update(index, patch) { setRules(current => current.map((rule, i) => i === index ? { ...rule, ...patch } : rule)); }
  async function save() {
    setBusy(true); setError(""); setNotice("");
    try { setRules(await api.automation("/rules", "PUT", rules)); setNotice("Rules saved."); }
    catch (e) { setError(e.message); } finally { setBusy(false); }
  }
  return <Card className="mb-3"><Card.Body>
    <h5>Automatic rules</h5>
    <p>Route matching events to an integration. Use an empty event subscription on the destination if it should receive only rule matches. Destination filters still apply.</p>
    {error && <Alert variant="danger">{error}</Alert>}
    {notice && <Alert variant="success">{notice}</Alert>}
    {rules.map((rule, index) => <div className="border rounded p-3 mb-2" key={rule.id}>
      <Form.Control className="mb-2" aria-label="Rule name" placeholder="Rule name" value={rule.name} onChange={e => update(index, { name: e.target.value })} />
      <Form.Control className="mb-2" aria-label="Rule event" placeholder="document.updated" value={rule.event} onChange={e => update(index, { event: e.target.value })} />
      <Form.Select className="mb-2" aria-label="Rule destination" value={rule.integration_id} onChange={e => update(index, { integration_id: e.target.value, user_id: integrations.find(i => i.id === e.target.value)?.user_id || "" })}>
        <option value="">Choose destination</option>{integrations.map(i => <option key={i.id} value={i.id}>{i.name}</option>)}
      </Form.Select>
      {rule.filters.map((f, fi) => <div className="d-flex gap-2 mb-2" key={fi}>
        <Form.Select aria-label="Filter field" value={f.field} onChange={e => update(index, { filters: rule.filters.map((v, j) => j === fi ? { ...v, field: e.target.value } : v) })}>{["document.name", "document.type", "document.id", "page.id", "content.text"].map(v => <option key={v}>{v}</option>)}</Form.Select>
        <Form.Select aria-label="Filter operator" value={f.operator} onChange={e => update(index, { filters: rule.filters.map((v, j) => j === fi ? { ...v, operator: e.target.value } : v) })}>{["equals", "not_equals", "contains", "starts_with", "exists"].map(v => <option key={v}>{v}</option>)}</Form.Select>
        <Form.Control aria-label="Filter value" disabled={f.operator === "exists"} value={f.value || ""} onChange={e => update(index, { filters: rule.filters.map((v, j) => j === fi ? { ...v, value: e.target.value } : v) })} />
        <Button variant="outline-danger" onClick={() => update(index, { filters: rule.filters.filter((_, j) => j !== fi) })}>Remove</Button>
      </div>)}
      <div className="d-flex gap-2 align-items-center"><Form.Check label="Enabled" checked={rule.enabled} onChange={e => update(index, { enabled: e.target.checked })} /><Button size="sm" variant="outline-secondary" onClick={() => update(index, { filters: [...rule.filters, { field: "document.name", operator: "starts_with", value: "" }] })}>Add filter</Button><Button size="sm" variant="outline-danger" onClick={() => setRules(rules.filter((_, i) => i !== index))}>Remove rule</Button></div>
    </div>)}
    <div className="d-flex gap-2"><Button variant="outline-secondary" disabled={busy} onClick={() => setRules([...rules, { id: `rule-${Date.now()}`, name: "", enabled: true, event: "document.updated", user_id: "", integration_id: "", filters: [] }])}>Add rule</Button><Button disabled={busy} onClick={save}>Save rules</Button></div>
  </Card.Body></Card>;
}
