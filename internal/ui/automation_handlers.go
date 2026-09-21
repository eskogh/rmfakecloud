package ui

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/ddvk/rmfakecloud/internal/automation"
	"github.com/gin-gonic/gin"
)

func (app *ReactAppWrapper) automationAvailable(c *gin.Context) {
	if app.cfg.Automation == nil {
		c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "Automation settings are unavailable"})
	}
}
func automationError(c *gin.Context, err error) {
	status := http.StatusBadRequest
	if errors.Is(err, automation.ErrNotFound) {
		status = http.StatusNotFound
	}
	c.AbortWithStatusJSON(status, gin.H{"error": err.Error()})
}
func (app *ReactAppWrapper) listAutomations(c *gin.Context) {
	c.JSON(http.StatusOK, app.cfg.Automation.List())
}
func (app *ReactAppWrapper) getAutomation(c *gin.Context) {
	v, err := app.cfg.Automation.Get(c.Param("id"))
	if err != nil {
		automationError(c, err)
		return
	}
	c.JSON(http.StatusOK, v)
}
func (app *ReactAppWrapper) saveAutomation(c *gin.Context) {
	var cfg automation.Config
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 64<<10)
	if err := c.ShouldBindJSON(&cfg); err != nil {
		badReq(c, "Invalid integration configuration")
		return
	}
	create := c.Request.Method == http.MethodPost
	if !create {
		cfg.ID = c.Param("id")
	}
	if cfg.UserID == "" {
		cfg.UserID = userID(c)
	}
	if cfg.UserID != userID(c) {
		if app.userStorer == nil {
			badReq(c, "Unknown integration user")
			return
		}
		if _, err := app.userStorer.GetUser(cfg.UserID); err != nil {
			badReq(c, "Unknown integration user")
			return
		}
	}
	view, err := app.cfg.Automation.Save(cfg, create)
	if err != nil {
		automationError(c, err)
		return
	}
	status := http.StatusOK
	if create {
		status = http.StatusCreated
	}
	c.JSON(status, view)
}
func (app *ReactAppWrapper) deleteAutomation(c *gin.Context) {
	if err := app.cfg.Automation.Delete(c.Param("id")); err != nil {
		automationError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
func (app *ReactAppWrapper) testAutomation(c *gin.Context) {
	if err := app.cfg.Automation.Test(c.Param("id")); err != nil {
		automationError(c, err)
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"message": "Test event queued; see delivery history"})
}
func (app *ReactAppWrapper) automationHistory(c *gin.Context) {
	h, err := app.cfg.Automation.History(c.Param("id"))
	if err != nil {
		automationError(c, err)
		return
	}
	c.JSON(http.StatusOK, h)
}

func (app *ReactAppWrapper) automationRules(c *gin.Context) {
	if c.Request.Method == http.MethodGet {
		c.JSON(http.StatusOK, app.cfg.Automation.Rules())
		return
	}
	var rules []automation.Rule
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 64<<10)
	if err := c.ShouldBindJSON(&rules); err != nil {
		badReq(c, "Invalid automation rules")
		return
	}
	if err := app.cfg.Automation.ReplaceRules(rules); err != nil {
		automationError(c, err)
		return
	}
	c.JSON(http.StatusOK, app.cfg.Automation.Rules())
}

// sendAutomation is an explicit admin action, separate from tablet sync routes.
// Multipart uploads are bounded and spooled by net/http; JSON sends text only.
func (app *ReactAppWrapper) sendAutomation(c *gin.Context) {
	view, err := app.cfg.Automation.Get(c.Param("id"))
	if err != nil {
		automationError(c, err)
		return
	}
	event := automation.NewEvent("document.send_requested", view.UserID)
	var attachment *automation.Attachment
	if strings.HasPrefix(c.GetHeader("Content-Type"), "multipart/form-data") {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, (25<<20)+(64<<10))
		err := c.Request.ParseMultipartForm(64 << 10)
		if c.Request.MultipartForm != nil {
			defer c.Request.MultipartForm.RemoveAll()
		}
		if err != nil {
			badReq(c, "Invalid upload or attachment exceeds 25 MiB")
			return
		}
		event.Data.Content = &automation.Content{Text: c.PostForm("text")}
		file, err := c.FormFile("attachment")
		if err != nil {
			badReq(c, "An attachment is required")
			return
		}
		attachment = &automation.Attachment{Name: file.Filename, ContentType: file.Header.Get("Content-Type"), Size: file.Size, Open: func(context.Context) (io.ReadCloser, error) { return file.Open() }}
	} else {
		var input struct {
			Text string `json:"text"`
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 64<<10)
		if err := c.ShouldBindJSON(&input); err != nil {
			badReq(c, "Invalid send request")
			return
		}
		event.Data.Content = &automation.Content{Text: input.Text}
	}
	if err := app.cfg.Automation.Send(c.Request.Context(), view.ID, view.UserID, event, attachment); err != nil {
		automationError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Sent"})
}
