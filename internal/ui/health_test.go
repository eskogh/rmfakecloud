package ui

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestHealthRequiresAdministrator(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, admin := range []bool{false, true} {
		app := &ReactAppWrapper{}
		router := gin.New()
		router.Use(func(c *gin.Context) { c.Set(AdminRole, admin) })
		router.GET("/health", app.adminMiddleware(), func(c *gin.Context) { c.Status(http.StatusOK) })
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest("GET", "/health", nil))
		want := http.StatusForbidden
		if admin {
			want = http.StatusOK
		}
		if response.Code != want {
			t.Fatalf("admin=%v: got %d, want %d", admin, response.Code, want)
		}
	}
}
