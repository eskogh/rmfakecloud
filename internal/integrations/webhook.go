package integrations

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"time"

	"github.com/ddvk/rmfakecloud/internal/messages"
	"github.com/ddvk/rmfakecloud/internal/model"
)

type Webhook struct {
	Endpoint   string
	Timeout    time.Duration
	Headers    map[string]string
	HMACSecret string
	HMACHeader string
}

func newWebhook(i model.IntegrationConfig) *Webhook {
	return &Webhook{
		Endpoint:   i.Endpoint,
		Timeout:    i.Timeout,
		Headers:    i.Headers,
		HMACSecret: i.HMACSecret,
		HMACHeader: i.HMACHeader,
	}
}

func (i *Webhook) SendMessage(data messages.IntegrationMessageData, img image.Image) (string, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Add data field
	if mdata, err := json.Marshal(data); err != nil {
		return "", err
	} else if err := writer.WriteField("data", string(mdata)); err != nil {
		return "", err
	}

	// Add attachment
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return "", err
	}

	part, err := writer.CreateFormFile("attachment", "reMarkable.png")
	if err != nil {
		return "", err
	}
	if _, err := part.Write(buf.Bytes()); err != nil {
		return "", err
	}

	// Close
	if err := writer.Close(); err != nil {
		return "", err
	}

	// Sign the exact bytes sent, including the multipart boundary and attachment.
	req, err := http.NewRequest(http.MethodPost, i.Endpoint, body)
	if err != nil {
		return "", fmt.Errorf("invalid webhook request")
	}
	for name, value := range i.Headers {
		req.Header.Set(name, value)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if i.HMACSecret != "" {
		header := i.HMACHeader
		if header == "" {
			header = "X-Webhook-Signature"
		}
		if http.CanonicalHeaderKey(header) == "Content-Type" {
			return "", fmt.Errorf("webhook signature header cannot be Content-Type")
		}
		mac := hmac.New(sha256.New, []byte(i.HMACSecret))
		mac.Write(body.Bytes())
		req.Header.Set(header, fmt.Sprintf("sha256=%x", mac.Sum(nil)))
	}
	timeout := i.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	if timeout < 0 {
		return "", fmt.Errorf("webhook timeout must be positive")
	}
	client := &http.Client{
		Timeout: timeout,
		// Do not forward credentials or signed payloads to redirect destinations.
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("webhook request failed (network, TLS, or timeout)")
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("webhook returned HTTP %d", resp.StatusCode)
	}

	responseData, err := io.ReadAll(io.LimitReader(resp.Body, (64<<10)+1))
	if err != nil {
		return "", err
	}

	if len(responseData) > 64<<10 {
		return "", fmt.Errorf("webhook response exceeds 64 KiB")
	}
	return string(responseData), nil
}
