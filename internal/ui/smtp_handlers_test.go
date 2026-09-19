package ui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ddvk/rmfakecloud/internal/config"
	"github.com/ddvk/rmfakecloud/internal/email"
	"github.com/gin-gonic/gin"
)

func TestSMTPSettingsAdminAndSecretRedaction(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store, err := email.OpenSettings(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	app := &ReactAppWrapper{cfg: &config.Config{SMTPSettings: store}}
	for _, admin := range []bool{false, true} {
		router := gin.New()
		router.Use(func(c *gin.Context) { c.Set(AdminRole, admin) })
		group := router.Group("", app.adminMiddleware())
		group.GET("/smtp", app.smtpSettings)
		group.PUT("/smtp", app.saveSMTPSettings)
		group.DELETE("/smtp", app.resetSMTPSettings)
		for _, method := range []string{"PUT", "GET", "DELETE"} {
			req := httptest.NewRequest(method, "/smtp", strings.NewReader(`{"server":"smtp.example:587","security":"starttls","password":"test-private-password"}`))
			req.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			router.ServeHTTP(response, req)
			want := http.StatusForbidden
			if admin {
				want = http.StatusOK
			}
			if response.Code != want {
				t.Fatalf("%s admin=%v: %d %s", method, admin, response.Code, response.Body.String())
			}
			if strings.Contains(response.Body.String(), "test-private-password") {
				t.Fatal("password in response")
			}
		}
	}
}

func TestSMTPTestAccessAndValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	app := &ReactAppWrapper{cfg: &config.Config{}}
	for _, tc := range []struct {
		admin  bool
		body   string
		status int
	}{
		{false, `{"to":"a@example.com"}`, http.StatusForbidden},
		{true, `{"to":"invalid"}`, http.StatusBadRequest},
		{true, `{"to":"a@example.com,b@example.com"}`, http.StatusBadRequest},
		{true, `{"to":"a@example.com"}`, http.StatusBadRequest},
	} {
		router := gin.New()
		router.Use(func(c *gin.Context) { c.Set(AdminRole, tc.admin) })
		router.POST("/test", app.adminMiddleware(), app.testSMTP)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest("POST", "/test", strings.NewReader(tc.body)))
		if response.Code != tc.status {
			t.Fatalf("got %d: %s", response.Code, response.Body.String())
		}
	}
}
