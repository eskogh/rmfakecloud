package ui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ddvk/rmfakecloud/internal/automation"
	"github.com/ddvk/rmfakecloud/internal/config"
	"github.com/gin-gonic/gin"
)

func TestAutomationAdminAndSecretMasking(t *testing.T) {
	gin.SetMode(gin.TestMode)
	bus := automation.NewBus(4, time.Second)
	defer bus.Close()
	manager, err := automation.OpenManager(t.TempDir(), bus, automation.Policy{})
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	app := &ReactAppWrapper{cfg: &config.Config{Automation: manager}}
	for _, admin := range []bool{false, true} {
		router := gin.New()
		router.Use(func(c *gin.Context) { c.Set(AdminRole, admin); c.Set(userIDContextKey, "user") })
		g := router.Group("", app.adminMiddleware(), app.automationAvailable)
		g.POST("/automation", app.saveAutomation)
		g.GET("/automation", app.listAutomations)
		req := httptest.NewRequest("POST", "/automation", strings.NewReader(`{"id":"demo","name":"Example","type":"webhook","endpoint":"https://example.com/secret-endpoint","enabled":true,"events":["document.*"],"auth":{"type":"bearer","token":"private-token"},"signing":{"enabled":true,"secret":"private-signing"},"headers":{"X-Key":"private-header"}}`))
		req.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, req)
		want := 403
		if admin {
			want = 201
		}
		if response.Code != want {
			t.Fatalf("%d %s", response.Code, response.Body.String())
		}
		if strings.Contains(response.Body.String(), "private-") || strings.Contains(response.Body.String(), "secret-endpoint") {
			t.Fatal("secret leaked")
		}
		response = httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest("GET", "/automation", nil))
		want = 403
		if admin {
			want = 200
		}
		if response.Code != want {
			t.Fatal(response.Code)
		}
	}
}

func TestAutomationRulesManualSendAndMissingManager(t *testing.T) {
	gin.SetMode(gin.TestMode)
	bus := automation.NewBus(8, time.Second)
	defer bus.Close()
	received := make(chan automation.Event, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var e automation.Event
		json.NewDecoder(r.Body).Decode(&e)
		received <- e
	}))
	defer server.Close()
	manager, err := automation.OpenManager(t.TempDir(), bus, automation.Policy{AllowHTTP: true, AllowPrivateNetworks: true})
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	_, err = manager.Save(automation.Config{ID: "demo", Name: "Demo", Type: "webhook", Enabled: true, UserID: "user", Endpoint: server.URL}, true)
	if err != nil {
		t.Fatal(err)
	}
	app := &ReactAppWrapper{cfg: &config.Config{Automation: manager}}
	for _, admin := range []bool{false, true} {
		router := gin.New()
		router.Use(func(c *gin.Context) { c.Set(AdminRole, admin); c.Set(userIDContextKey, "user") })
		g := router.Group("/automation", app.adminMiddleware(), app.automationAvailable)
		g.GET("/rules", app.automationRules)
		g.PUT("/rules", app.automationRules)
		g.POST("/:id/send", app.sendAutomation)
		g.DELETE("/:id", app.deleteAutomation)
		g.GET("/:id/deliveries", app.automationHistory)
		requests := []struct {
			method, path, body string
			status             int
		}{
			{"PUT", "/automation/rules", `[{"id":"work","name":"Work","user_id":"user","enabled":true,"event":"document.*","filters":[],"integration_id":"demo"}]`, 200},
			{"GET", "/automation/rules", "", 200},
			{"POST", "/automation/demo/send", `{"text":"Hello"}`, 200},
			{"GET", "/automation/demo/deliveries", "", 200},
			{"DELETE", "/automation/demo", "", 400},
		}
		for _, tc := range requests {
			r := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			r.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, r)
			want := tc.status
			if !admin {
				want = 403
			}
			if w.Code != want {
				t.Fatalf("%s %s: %d %s", tc.method, tc.path, w.Code, w.Body.String())
			}
		}
	}
	select {
	case e := <-received:
		if e.Event != "document.send_requested" || e.Data.User.ID != "user" || e.Data.Content.Text != "Hello" {
			t.Fatal(e)
		}
	case <-time.After(time.Second):
		t.Fatal("manual send missing")
	}
	app.cfg.Automation = nil
	router := gin.New()
	router.GET("/automation", app.automationAvailable, app.listAutomations)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest("GET", "/automation", nil))
	if w.Code != 503 {
		t.Fatal(w.Code)
	}
}
