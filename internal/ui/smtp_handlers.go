package ui

import (
	"context"
	"errors"
	log "github.com/sirupsen/logrus"
	"net/http"
	"net/mail"
	"os"
	"strings"
	"time"

	"github.com/ddvk/rmfakecloud/internal/email"
	"github.com/gin-gonic/gin"
)

func (app *ReactAppWrapper) smtpSettings(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	if app.cfg.SMTPSettings == nil {
		c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "SMTP settings are unavailable"})
		return
	}
	c.JSON(http.StatusOK, app.cfg.SMTPSettings.View())
}
func (app *ReactAppWrapper) saveSMTPSettings(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	if app.cfg.SMTPSettings == nil {
		c.AbortWithStatus(http.StatusServiceUnavailable)
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 16<<10)
	var request struct {
		email.Settings
		ClearPassword bool `json:"clearPassword"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		badReq(c, "Invalid SMTP settings")
		return
	}
	if err := app.cfg.SMTPSettings.Save(request.Settings, request.ClearPassword); err != nil {
		var pathErr *os.PathError
		if errors.As(err, &pathErr) {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Could not persist SMTP settings"})
			return
		}
		badReq(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, app.cfg.SMTPSettings.View())
}
func (app *ReactAppWrapper) resetSMTPSettings(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	if app.cfg.SMTPSettings == nil {
		c.AbortWithStatus(http.StatusServiceUnavailable)
		return
	}
	if err := app.cfg.SMTPSettings.Reset(); err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Could not remove saved SMTP settings"})
		return
	}
	c.JSON(http.StatusOK, app.cfg.SMTPSettings.View())
}

// testSMTP sends one fixed message using the active configuration, never a browser-supplied server.
func (app *ReactAppWrapper) testSMTP(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 4096)
	var request struct {
		To   string `json:"to"`
		From string `json:"from"`
	}
	if c.ShouldBindJSON(&request) != nil {
		badReq(c, "Enter a valid test recipient and sender")
		return
	}
	to, err := mail.ParseAddress(request.To)
	if err != nil || strings.ContainsAny(request.To+request.From, "\r\n") {
		badReq(c, "Enter a single valid recipient without line breaks")
		return
	}
	cfg := app.cfg.SMTPForRequest(c.Request)
	if cfg == nil {
		badReq(c, "Configure and save SMTP settings first")
		return
	}
	from := cfg.FromOverride
	if from == nil {
		from, err = mail.ParseAddress(request.From)
		if err != nil {
			badReq(c, "Enter a valid test sender or configure a sender address override")
			return
		}
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()
	message := &email.Builder{From: from, To: []*mail.Address{to}, Subject: "rmfakecloud SMTP test", Body: "<p>Your rmfakecloud SMTP test message was sent successfully.</p>"}
	if err := message.SendContext(ctx, cfg); err != nil {
		log.WithError(err).Warn("SMTP test failed")
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	log.Info("SMTP test accepted by mail server")
	c.JSON(http.StatusOK, gin.H{"message": "SMTP server accepted the test email. Check your inbox and spam folder; acceptance does not guarantee delivery."})
}
