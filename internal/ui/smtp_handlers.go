package ui

import (
	"errors"
	"net/http"
	"os"

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
