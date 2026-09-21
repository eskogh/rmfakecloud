package integrations

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"fmt"
	"image"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ddvk/rmfakecloud/internal/messages"
	"github.com/ddvk/rmfakecloud/internal/model"
	"gopkg.in/yaml.v3"
)

func TestWebhookStatus(t *testing.T) {
	for _, status := range []int{200, 201, 204, 299, 302, 400, 500} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Location", "/redirect")
				w.WriteHeader(status)
				if status != 204 {
					fmt.Fprint(w, "message-id")
				}
			}))
			defer server.Close()
			id, err := (&Webhook{Endpoint: server.URL}).SendMessage(messages.IntegrationMessageData{}, image.NewRGBA(image.Rect(0, 0, 1, 1)))
			if status >= 300 {
				if err == nil || id != "" {
					t.Fatalf("got id %q, error %v", id, err)
				}
			} else if err != nil || (status != 204 && id != "message-id") || (status == 204 && id != "") {
				t.Fatalf("got id %q, error %v", id, err)
			}
		})
	}
}

func TestWebhookAuthenticationAndPayload(t *testing.T) {
	for _, header := range []string{"", "X-Custom-Signature"} {
		t.Run(header, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				raw, err := io.ReadAll(r.Body)
				if err != nil {
					t.Error(err)
					return
				}
				signatureHeader := header
				if signatureHeader == "" {
					signatureHeader = "X-Webhook-Signature"
				}
				mac := hmac.New(sha256.New, []byte("secret"))
				mac.Write(raw)
				if got := r.Header.Get(signatureHeader); got != fmt.Sprintf("sha256=%x", mac.Sum(nil)) {
					t.Errorf("invalid signature: %q", got)
				}
				if r.Header.Get("Authorization") != "Bearer token" {
					t.Error("missing authentication")
				}
				r.Body = io.NopCloser(bytes.NewReader(raw))
				if err := r.ParseMultipartForm(1 << 20); err != nil {
					t.Error(err)
					return
				}
				defer r.MultipartForm.RemoveAll()
				if r.FormValue("data") == "" {
					t.Error("missing data")
				}
				file, _, err := r.FormFile("attachment")
				if err != nil {
					t.Error(err)
					return
				}
				defer file.Close()
				if _, _, err := image.Decode(file); err != nil {
					t.Error(err)
				}
				fmt.Fprint(w, "signed-id")
			}))
			defer server.Close()
			var config model.IntegrationConfig
			if err := yaml.Unmarshal([]byte("timeout: 1s\nheaders:\n  Authorization: Bearer token\nhmacsecret: secret\n"), &config); err != nil {
				t.Fatal(err)
			}
			config.Endpoint = server.URL
			config.HMACHeader = header
			id, err := newWebhook(config).SendMessage(messages.IntegrationMessageData{}, image.NewRGBA(image.Rect(0, 0, 1, 1)))
			if err != nil || id != "signed-id" {
				t.Fatalf("got %q, %v", id, err)
			}
		})
	}
}

func TestWebhookTimeout(t *testing.T) {
	for _, flush := range []bool{false, true} {
		t.Run(fmt.Sprint(flush), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				io.Copy(io.Discard, r.Body)
				if flush {
					w.WriteHeader(http.StatusOK)
					w.(http.Flusher).Flush()
				}
				select {
				case <-r.Context().Done():
				case <-time.After(time.Second):
				}
			}))
			defer server.Close()
			id, err := (&Webhook{Endpoint: server.URL, Timeout: 30 * time.Millisecond}).SendMessage(messages.IntegrationMessageData{}, image.NewRGBA(image.Rect(0, 0, 1, 1)))
			if err == nil || id != "" {
				t.Fatalf("got %q, %v", id, err)
			}
		})
	}
}

func TestWebhookResponseBoundAndSafeErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(bytes.Repeat([]byte("x"), (64<<10)+1))
	}))
	defer server.Close()
	_, err := (&Webhook{Endpoint: server.URL}).SendMessage(messages.IntegrationMessageData{}, image.NewRGBA(image.Rect(0, 0, 1, 1)))
	if err == nil {
		t.Fatal("accepted oversized response")
	}
	_, err = (&Webhook{Endpoint: "http://%private-token"}).SendMessage(messages.IntegrationMessageData{}, image.NewRGBA(image.Rect(0, 0, 1, 1)))
	if err == nil || strings.Contains(err.Error(), "private-token") {
		t.Fatal("unsafe URL error", err)
	}
}
