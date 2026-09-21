package automation

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// Native shares delivery accounting, retries, timeouts and network policy with
// webhooks. It never downloads URLs found in events.
type Native struct{ *Webhook }

func newIntegration(c Config, p Policy, record Recorder) (Integration, error) {
	w, err := NewWebhook(c, p, record)
	if err != nil {
		return nil, err
	}
	if c.Type == "webhook" {
		return w, nil
	}
	return &Native{w}, nil
}
func (n *Native) HandleEvent(ctx context.Context, e Event) error { return n.Send(ctx, e, nil) }
func (n *Native) Send(ctx context.Context, e Event, attachment *Attachment) error {
	if attachment != nil && (attachment.Open == nil || attachment.Size < 0 || attachment.Size > 25<<20) {
		return errors.New("attachment must have an opener and size between 0 and 25 MiB")
	}
	return n.deliver(ctx, e, func(_ string) (int, bool, error) { return n.sendAttempt(ctx, e, attachment) })
}
func notification(e Event, limit int) string {
	text := e.Event
	if e.Data.Document != nil {
		text += ": " + e.Data.Document.Name
	}
	if e.Data.Content != nil && e.Data.Content.Text != "" {
		text += "\n" + e.Data.Content.Text
	}
	if e.Data.Test {
		text = "rmfakecloud integration test"
	}
	chars := []rune(text)
	if len(chars) > limit {
		text = string(chars[:limit])
	}
	return text
}
func (n *Native) sendAttempt(ctx context.Context, e Event, attachment *Attachment) (int, bool, error) {
	endpoint := n.config.Endpoint
	fields := map[string]string{}
	var payload any
	fileField := "files[0]"
	if n.Type() == "telegram" {
		token, err := secret(n.config.Auth.Token)
		if err != nil {
			return 0, false, err
		}
		// Tokens are a path component; reject delimiters instead of permitting URL injection.
		if strings.ContainsAny(token, "/?#%\r\n ") {
			return 0, false, errors.New("invalid bot token")
		}
		method := "sendMessage"
		fields["chat_id"] = n.config.ChatID
		if attachment == nil {
			fields["text"] = notification(e, 4000)
		} else {
			method, fileField = "sendDocument", "document"
			if attachment.ContentType == "image/png" {
				method, fileField = "sendPhoto", "photo"
			}
			fields["caption"] = notification(e, 1000)
		}
		endpoint = "https://api.telegram.org/bot" + token + "/" + method
		payload = fields
	} else {
		u, err := url.Parse(endpoint)
		if err != nil {
			return 0, false, errors.New("invalid Discord endpoint")
		}
		query := u.Query()
		query.Set("wait", "true")
		u.RawQuery = query.Encode()
		endpoint = u.String()
		payload = struct {
			Content         string `json:"content"`
			AllowedMentions struct {
				Parse []string `json:"parse"`
			} `json:"allowed_mentions"`
		}{Content: notification(e, 1900), AllowedMentions: struct {
			Parse []string `json:"parse"`
		}{Parse: []string{}}}
	}
	data, _ := json.Marshal(payload)
	var body io.Reader = bytes.NewReader(data)
	contentType := "application/json"
	if attachment != nil {
		// Spool multipart to a private temporary file: bounded RAM and a fresh stream
		// on retries, with no upload goroutine left behind after cancellation.
		file, err := os.CreateTemp("", "rmfakecloud-upload-*")
		if err != nil {
			return 0, false, errors.New("could not prepare attachment")
		}
		defer os.Remove(file.Name())
		defer file.Close()
		writer := multipart.NewWriter(file)
		if n.Type() == "discord" {
			fields["payload_json"] = string(data)
		}
		for key, value := range fields {
			if err := writer.WriteField(key, value); err != nil {
				return 0, false, errors.New("could not prepare attachment")
			}
		}
		part, err := writer.CreateFormFile(fileField, filepath.Base(attachment.Name))
		if err != nil {
			return 0, false, errors.New("invalid attachment name")
		}
		src, err := attachment.Open(ctx)
		if err != nil {
			return 0, false, errors.New("could not open attachment")
		}
		stop := context.AfterFunc(ctx, func() { src.Close() })
		count, copyErr := io.Copy(part, io.LimitReader(src, (25<<20)+1))
		stop()
		src.Close()
		if copyErr != nil || count > 25<<20 || count != attachment.Size {
			return 0, false, errors.New("attachment read failed or size mismatch")
		}
		if writer.Close() != nil {
			return 0, false, errors.New("could not prepare attachment")
		}
		if _, err := file.Seek(0, 0); err != nil {
			return 0, false, errors.New("could not prepare attachment")
		}
		body, contentType = file, writer.FormDataContentType()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, body)
	if err != nil {
		return 0, false, errors.New("invalid native integration request")
	}
	req.Header.Set("Content-Type", contentType)
	resp, err := n.client.Do(req)
	if err != nil {
		return 0, true, errors.New("integration request failed (network, TLS, timeout, or policy)")
	}
	defer resp.Body.Close()
	response, readErr := io.ReadAll(io.LimitReader(resp.Body, (64<<10)+1))
	status := resp.StatusCode
	if status < 200 || status >= 300 {
		return status, status == 408 || status == 429 || status >= 500, fmt.Errorf("integration returned HTTP %d", status)
	}
	if n.Type() == "telegram" {
		var result struct {
			OK bool `json:"ok"`
		}
		if readErr != nil || len(response) > 64<<10 || json.Unmarshal(response, &result) != nil || !result.OK {
			return status, false, errors.New("Telegram did not confirm delivery")
		}
	}
	return status, false, nil
}
