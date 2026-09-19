import { useEffect, useState } from "react";
import { Alert, Button, Form, Modal } from "react-bootstrap";
import useResource from "../../hooks/useResource";
import api from "../../services/api.service";

export default function MailSettings() {
  const { data, error, loading } = useResource("settings/smtp");
  const [form, setForm] = useState(null);
  const [source, setSource] = useState("unconfigured");
  const [busy, setBusy] = useState(false);
  const [saveError, setSaveError] = useState("");
  const [message, setMessage] = useState("");
  const [confirmReset, setConfirmReset] = useState(false);
  const apply = (value) => {
    setForm({ ...value, password: "", clearPassword: false });
    setSource(value.source);
  };
  useEffect(() => {
    if (data) apply(data);
  }, [data]);
  const change = (key, value) => {
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
            <fieldset disabled={busy}>
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
