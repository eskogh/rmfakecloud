package automation

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestNativeMessagesAndAttachments(t *testing.T) {
	for _, kind := range []string{"telegram", "discord"} {
		for _, media := range []string{"", "image/png", "application/pdf"} {
			t.Run(kind+media, func(t *testing.T) {
				c := testConfig("https://discord.com/api/webhooks/id/token")
				c.Type = kind
				if kind == "telegram" {
					c.Endpoint = ""
					c.ChatID = "42"
					c.Auth.Token = "123:token"
				}
				var recorded Delivery
				integration, err := newIntegration(c, Policy{}, func(d Delivery) { recorded = d })
				if err != nil {
					t.Fatal(err)
				}
				n := integration.(*Native)
				defer n.Close()
				n.client.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
					if kind == "discord" && r.URL.Query().Get("wait") != "true" {
						t.Error("Discord delivery is not confirmed")
					}
					if media == "" {
						var payload map[string]any
						if json.NewDecoder(r.Body).Decode(&payload) != nil {
							t.Fatal("invalid JSON")
						}
						if kind == "telegram" && (payload["chat_id"] != "42" || payload["text"] == "") {
							t.Error(payload)
						}
						if kind == "discord" && payload["allowed_mentions"] == nil {
							t.Error("mentions not disabled")
						}
					} else {
						if err := r.ParseMultipartForm(1024); err != nil {
							t.Fatal(err)
						}
						defer r.MultipartForm.RemoveAll()
						field := "files[0]"
						if kind == "telegram" {
							field = "document"
							if media == "image/png" {
								field = "photo"
							}
						}
						file, _, err := r.FormFile(field)
						if err != nil {
							t.Fatal(err)
						}
						defer file.Close()
						data, _ := io.ReadAll(file)
						if string(data) != "file contents" {
							t.Fatal("attachment corrupted")
						}
						if kind == "telegram" && r.FormValue("chat_id") != "42" {
							t.Error("missing chat")
						}
					}
					return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"ok":true}`)), Header: http.Header{}}, nil
				})
				var a *Attachment
				if media != "" {
					a = &Attachment{Name: "page", ContentType: media, Size: 13, Open: func(context.Context) (io.ReadCloser, error) {
						return io.NopCloser(strings.NewReader("file contents")), nil
					}}
				}
				if err := n.Send(context.Background(), NewEvent("document.updated", "user"), a); err != nil {
					t.Fatal(err)
				}
				if !recorded.Success || recorded.Attempts != 1 {
					t.Fatal(recorded)
				}
			})
		}
	}
}
func TestNativeRetriesReopenAttachment(t *testing.T) {
	attempts, opens := 0, 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)
		attempts++
		if attempts == 1 {
			w.WriteHeader(429)
		}
	}))
	defer server.Close()
	c := testConfig(server.URL)
	c.Type = "discord"
	c.Retry = Retry{Count: 1, Delays: []string{"0s"}}
	i, err := newIntegration(c, Policy{true, true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	n := i.(*Native)
	defer n.Close()
	a := &Attachment{Name: "page.pdf", Size: 4, Open: func(context.Context) (io.ReadCloser, error) {
		opens++
		return io.NopCloser(strings.NewReader("test")), nil
	}}
	if err := n.Send(context.Background(), NewEvent("document.send_requested", "user"), a); err != nil {
		t.Fatal(err)
	}
	if opens != 2 || attempts != 2 {
		t.Fatal(opens, attempts)
	}
}
func TestTelegramFailureAndTokenValidation(t *testing.T) {
	c := testConfig("")
	c.Type = "telegram"
	c.ChatID = "42"
	c.Auth.Token = "123:token"
	i, err := newIntegration(c, Policy{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	n := i.(*Native)
	defer n.Close()
	n.client.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"ok":false,"description":"secret"}`))}, nil
	})
	if err := n.HandleEvent(context.Background(), NewEvent("integration.test", "user")); err == nil || strings.Contains(err.Error(), "secret") {
		t.Fatal(err)
	}
	n.config.Auth.Token = "token/../../elsewhere"
	if err := n.HandleEvent(context.Background(), NewEvent("integration.test", "user")); err == nil {
		t.Fatal("accepted URL injection")
	}
}
