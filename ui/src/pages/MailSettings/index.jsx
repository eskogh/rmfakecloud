import { useEffect, useState } from "react";
import { Alert, Button, Form, Modal } from "react-bootstrap";
import useResource from "../../hooks/useResource";
import api from "../../services/api.service";

export default function MailSettings() {
  const { data, error, loading } = useResource("settings/smtp");
  const [testing, setTesting] = useState(false);
  const [dirty, setDirty] = useState(false);
  const [recipient, setRecipient] = useState("");
  const [sender, setSender] = useState("");
  const [testResult, setTestResult] = useState(null);
  const [activeFrom, setActiveFrom] = useState("");
  const [form, setForm] = useState(null);
  const [source, setSource] = useState("unconfigured");
  const [busy, setBusy] = useState(false);
  const [saveError, setSaveError] = useState("");
  const [message, setMessage] = useState("");
  const [confirmReset, setConfirmReset] = useState(false);
  const apply = (value) => {
    setForm({ ...value, password: "", clearPassword: false });
    setSource(value.source);
    setActiveFrom(value.from);
    setDirty(false);
    setTestResult(null);
  };
  useEffect(() => {
    if (data) apply(data);
  }, [data]);
  const change = (key, value) => {
    setDirty(true);
    setTestResult(null);
    setForm((v) => ({ ...v, [key]: value }));
    setMessage("");
  };
  const save = async (event) => {
    event.preventDefault();
    setBusy(true);
    setSaveError("");
    setMessage("");
    const {
      server,
      username,
      password,
      from,
      helo,
      security,
      insecureTLS,
      clearPassword,
    } = form;
    try {
      apply(
        await api.saveSMTP({
          server,
          username,
          password,
          from,
          helo,
          security,
          insecureTLS,
          clearPassword,
        }),
      );
      setMessage(
        "SMTP settings saved. New email requests use them immediately.",
      );
    } catch (e) {
      setSaveError(e.message);
    } finally {
      setBusy(false);
    }
  };
  const reset = async () => {
    setBusy(true);
    setSaveError("");
    setMessage("");
    try {
      apply(await api.resetSMTP());
      setConfirmReset(false);
      setMessage(
        "Saved override removed. Environment settings are now active, if configured.",
      );
    } catch (e) {
      setSaveError(e.message);
      setConfirmReset(false);
    } finally {
      setBusy(false);
    }
  };
  const test = async (event) => {
    event.preventDefault();
    setTesting(true);
    setTestResult(null);
    try {
      const result = await api.testSMTP({ to: recipient, from: sender });
      setTestResult({ ok: true, text: result.message });
    } catch (e) {
      setTestResult({ ok: false, text: e.message });
    } finally {
      setTesting(false);
    }
  };
  return (
    <main className="workspace-page mail-settings">
      <header className="page-heading">
        <div>
          <p className="eyebrow">INSTANCE ADMINISTRATION</p>
          <h1>Send from your reMarkable.</h1>
          <p className="muted">
            Configure the SMTP server used by every account on this instance.
          </p>
        </div>
      </header>
      {error && <Alert variant="danger">{error}</Alert>}
      {saveError && <Alert variant="danger">{saveError}</Alert>}
      {message && (
        <Alert variant="success" role="status">
          {message}
        </Alert>
      )}
      {loading && <p role="status">Loading mail settings…</p>}
      {form && (
        <section className="panel">
          <div className="panel-heading">
            <div>
              <h2>SMTP configuration</h2>
              <p className="muted">
                Active source:{" "}
                {source === "web"
                  ? "Saved web settings"
                  : source === "environment"
                    ? "Environment variables"
                    : "Not configured"}
              </p>
            </div>
          </div>
          <Form onSubmit={save} className="smtp-form">
            <fieldset disabled={busy || testing}>
              <div className="smtp-fields">
                <Form.Group controlId="smtp-server">
                  <Form.Label>SMTP server and port</Form.Label>
                  <Form.Control
                    required
                    value={form.server}
                    placeholder="smtp.example.com:587"
                    autoComplete="off"
                    onChange={(e) => change("server", e.target.value)}
                  />
                </Form.Group>
                <Form.Group controlId="smtp-security">
                  <Form.Label>Connection security</Form.Label>
                  <Form.Select
                    value={form.security}
                    onChange={(e) => change("security", e.target.value)}
                  >
                    <option value="starttls">
                      STARTTLS (usually port 587)
                    </option>
                    <option value="tls">TLS (usually port 465)</option>
                    <option value="none">None (trusted local relay)</option>
                  </Form.Select>
                </Form.Group>
                <Form.Group controlId="smtp-username">
                  <Form.Label>Username</Form.Label>
                  <Form.Control
                    value={form.username}
                    autoComplete="off"
                    onChange={(e) => change("username", e.target.value)}
                  />
                  <Form.Text>
                    Leave empty for a relay that does not require
                    authentication.
                  </Form.Text>
                </Form.Group>
                <Form.Group controlId="smtp-password">
                  <Form.Label>Password</Form.Label>
                  <Form.Control
                    type="password"
                    value={form.password}
                    disabled={form.clearPassword}
                    autoComplete="new-password"
                    placeholder={
                      form.hasPassword
                        ? "Password saved — leave blank to keep it"
                        : "SMTP password or app password"
                    }
                    onChange={(e) => change("password", e.target.value)}
                  />
                  <Form.Check
                    id="smtp-clear-password"
                    label="Clear the saved password"
                    checked={form.clearPassword}
                    onChange={(e) => {
                      change("clearPassword", e.target.checked);
                      change("password", "");
                    }}
                  />
                </Form.Group>
                <Form.Group controlId="smtp-from">
                  <Form.Label>Sender address override</Form.Label>
                  <Form.Control
                    value={form.from}
                    placeholder="My reMarkable <notes@example.com>"
                    onChange={(e) => change("from", e.target.value)}
                  />
                  <Form.Text>
                    Optional. Use an address allowed by your mail provider.
                    Replies go to the original sender.
                  </Form.Text>
                </Form.Group>
                <Form.Group controlId="smtp-helo">
                  <Form.Label>HELO hostname</Form.Label>
                  <Form.Control
                    value={form.helo}
                    placeholder="Optional"
                    onChange={(e) => change("helo", e.target.value)}
                  />
                </Form.Group>
              </div>
              <Form.Check
                id="smtp-insecure"
                label="Skip TLS certificate verification (for a trusted self-signed server)"
                checked={form.insecureTLS}
                onChange={(e) => change("insecureTLS", e.target.checked)}
              />
              <p className="chart-note">
                Saved settings override RM_SMTP_* environment variables and
                persist in the data directory. The password is stored in a file
                readable only by its owner and is never returned to the browser.
              </p>
              <div className="heading-actions">
                <Button type="submit">
                  {busy ? "Saving…" : "Save SMTP settings"}
                </Button>
                {source === "web" && (
                  <Button
                    variant="outline-secondary"
                    onClick={() => setConfirmReset(true)}
                  >
                    Use environment settings
                  </Button>
                )}
              </div>
            </fieldset>
          </Form>
          <p className="chart-note">
            After enabling email for the first time, reconnect or restart your
            tablet so it can refresh its mail capability. Save does not send a
            test message or verify delivery.
          </p>
        </section>
      )}
      {form && (
        <section className="panel">
          <div className="panel-heading"><div>
            <h2>Test email delivery</h2>
            <p className="muted">Sends one email using the active saved or environment settings. The test can take up to 30 seconds.</p>
          </div></div>
          {dirty && <Alert variant="warning">Save your changes before testing.</Alert>}
          {testResult && <Alert variant={testResult.ok ? "success" : "danger"} role={testResult.ok ? "status" : "alert"} style={{ overflowWrap: "anywhere", whiteSpace: "pre-wrap" }}>{testResult.text}</Alert>}
          <Form onSubmit={test} className="smtp-form">
            <fieldset disabled={testing || busy || dirty || source === "unconfigured"}>
              <div className="smtp-fields">
                <Form.Group controlId="smtp-test-to">
                  <Form.Label>Test recipient</Form.Label>
                  <Form.Control type="email" required value={recipient} onChange={(e) => { setRecipient(e.target.value); setTestResult(null); }} placeholder="you@example.com" />
                </Form.Group>
                {!activeFrom && <Form.Group controlId="smtp-test-from">
                  <Form.Label>Test sender</Form.Label>
                  <Form.Control type="email" required value={sender} onChange={(e) => { setSender(e.target.value); setTestResult(null); }} placeholder="notes@example.com" />
                  <Form.Text>Use an address your provider permits. This does not change your saved settings.</Form.Text>
                </Form.Group>}
              </div>
              {activeFrom && <p className="muted">Sender: {activeFrom}</p>}
              <Button type="submit">{testing ? "Sending test email…" : "Send test email"}</Button>
            </fieldset>
          </Form>
          <p className="chart-note">A successful test means the SMTP server accepted the message. If it does not arrive, check spam and your mail provider’s delivery logs.</p>
        </section>
      )}
      <Modal
        show={confirmReset}
        onHide={() => !busy && setConfirmReset(false)}
        centered
      >
        <Modal.Header closeButton={!busy}>
          <Modal.Title>Use environment settings?</Modal.Title>
        </Modal.Header>
        <Modal.Body>
          This removes the saved SMTP override and password. If SMTP is not
          configured through environment variables, email sending will be
          unavailable.
        </Modal.Body>
        <Modal.Footer>
          <Button
            variant="secondary"
            disabled={busy}
            onClick={() => setConfirmReset(false)}
          >
            Cancel
          </Button>
          <Button disabled={busy} onClick={reset}>
            Use environment settings
          </Button>
        </Modal.Footer>
      </Modal>
    </main>
  );
}
