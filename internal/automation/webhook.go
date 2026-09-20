package automation

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
)

type Delivery struct {
	ID            string    `json:"id"`
	IntegrationID string    `json:"integration_id"`
	Event         string    `json:"event"`
	Timestamp     time.Time `json:"timestamp"`
	HTTPStatus    int       `json:"http_status"`
	DurationMS    int64     `json:"duration_ms"`
	Attempts      int       `json:"attempts"`
	Success       bool      `json:"success"`
	Error         string    `json:"error,omitempty"`
}
type Recorder func(Delivery)
type Webhook struct {
	config Config
	client *http.Client
	record Recorder
}

func NewWebhook(c Config, p Policy, record Recorder) (*Webhook, error) {
	if err := c.Validate(p); err != nil {
		return nil, err
	}
	return &Webhook{config: c, client: p.client(c.duration()), record: record}, nil
}
func (w *Webhook) ID() string                 { return w.config.ID }
func (w *Webhook) Name() string               { return w.config.Name }
func (w *Webhook) Type() string               { return w.config.Type }
func (w *Webhook) Enabled() bool              { return w.config.Enabled }
func (w *Webhook) SubscribedEvents() []string { return append([]string(nil), w.config.Events...) }
func (w *Webhook) Close()                     { w.client.CloseIdleConnections() }
func secret(value string) (string, error) {
	if strings.HasPrefix(value, "${") && strings.HasSuffix(value, "}") {
		resolved, ok := os.LookupEnv(value[2 : len(value)-1])
		if !ok || resolved == "" {
			return "", errors.New("configured secret environment variable is unavailable")
		}
		return resolved, nil
	}
	return value, nil
}
func (w *Webhook) HandleEvent(ctx context.Context, e Event) (err error) {
	body, err := json.Marshal(e)
	if err != nil {
		return errors.New("event encoding failed")
	}
	delivery := Delivery{ID: uuid.NewString(), IntegrationID: w.ID(), Event: e.Event, Timestamp: time.Now().UTC()}
	start := time.Now()
	defer func() {
		delivery.DurationMS = time.Since(start).Milliseconds()
		delivery.Success = err == nil
		if err != nil {
			delivery.Error = err.Error()
		}
		if w.record != nil {
			w.record(delivery)
		}
		fields := log.Fields{"integration_id": w.ID(), "integration_type": w.Type(), "event": e.Event, "delivery_id": delivery.ID, "attempt": delivery.Attempts, "duration_ms": delivery.DurationMS, "status": delivery.HTTPStatus, "success": delivery.Success}
		log.WithFields(fields).Info("Automation delivery completed")
	}()
	delays := []time.Duration{5 * time.Second, 30 * time.Second, 2 * time.Minute}
	for idx, v := range w.config.Retry.Delays {
		delays[idx], _ = time.ParseDuration(v)
	}
	for attempt := 0; attempt <= w.config.Retry.Count; attempt++ {
		if attempt > 0 {
			timer := time.NewTimer(delays[attempt-1])
			select {
			case <-ctx.Done():
				timer.Stop()
				return errors.New("delivery canceled or timed out")
			case <-timer.C:
			}
		}
		delivery.Attempts++
		status, retry, attemptErr := w.attempt(ctx, e, delivery.ID, body)
		delivery.HTTPStatus = status
		if attemptErr == nil {
			return nil
		}
		err = attemptErr
		if !retry || ctx.Err() != nil {
			return err
		}
	}
	return err
}
func (w *Webhook) attempt(ctx context.Context, e Event, id string, body []byte) (int, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.config.Endpoint, bytes.NewReader(body))
	if err != nil {
		return 0, false, errors.New("invalid webhook request")
	}
	for key, value := range w.config.Headers {
		v, err := secret(value)
		if err != nil {
			return 0, false, err
		}
		req.Header.Set(key, v)
	}
	if w.config.Auth.Type == "bearer" {
		token, err := secret(w.config.Auth.Token)
		if err != nil {
			return 0, false, err
		}
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-RMFakeCloud-Event", e.Event)
	req.Header.Set("X-RMFakeCloud-Delivery", id)
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	req.Header.Set("X-RMFakeCloud-Timestamp", timestamp)
	if w.config.Signing.Enabled {
		key, err := secret(w.config.Signing.Secret)
		if err != nil {
			return 0, false, err
		}
		mac := hmac.New(sha256.New, []byte(key))
		mac.Write([]byte(timestamp + "."))
		mac.Write(body)
		req.Header.Set("X-RMFakeCloud-Signature", fmt.Sprintf("sha256=%x", mac.Sum(nil)))
	}
	resp, err := w.client.Do(req)
	if err != nil {
		return 0, true, errors.New("webhook request failed (network, TLS, timeout, or policy)")
	}
	defer resp.Body.Close()
	// Drain only a bounded amount; no remote body is included in logs or history.
	_, readErr := io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
	status := resp.StatusCode
	if status >= 200 && status < 300 {
		return status, false, nil
	} // acceptance is enough; do not replay after a body read failure
	_ = readErr
	return status, status == 408 || status == 429 || status >= 500, fmt.Errorf("webhook returned HTTP %d", status)
}
