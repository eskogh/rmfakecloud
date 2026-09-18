package ui

import (
	"context"
	"crypto/x509"
	"io/fs"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/ddvk/rmfakecloud/internal/common"
	"github.com/ddvk/rmfakecloud/internal/ui/viewmodel"
	"github.com/gin-gonic/gin"
)

func (app *ReactAppWrapper) dashboard(c *gin.Context) {
	tree, err := app.getBackend(c).GetDocumentTree(userID(c))
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Could not read the document library"})
		return
	}
	counts := map[string]int{"PDF": 0, "EPUB": 0, "Notebook": 0, "Other": 0}
	var bytes int64
	folders := 0
	documents := 0
	var visit func([]viewmodel.Entry)
	visit = func(entries []viewmodel.Entry) {
		for _, entry := range entries {
			switch d := entry.(type) {
			case *viewmodel.Directory:
				folders++
				visit(d.Entries)
			case *viewmodel.Document:
				documents++
				bytes += d.Size
				kind := "Other"
				switch strings.ToLower(d.DocumentType) {
				case "pdf":
					kind = "PDF"
				case "epub":
					kind = "EPUB"
				case "notebook", "":
					kind = "Notebook"
				}
				counts[kind]++
			}
		}
	}
	visit(tree.Entries)
	clients, activity, started := app.h.Metrics(userID(c), false)
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, gin.H{"documents": documents, "folders": folders, "documentBytes": bytes, "types": counts, "clients": clients, "activity": activity, "observedSince": started, "checkedAt": time.Now().UTC()})
}

// Health is administrator-only. External TLS termination cannot be inspected here.
func (app *ReactAppWrapper) health(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()
	version := app.cfg.Version
	if version == "" {
		version = "Development build"
	}
	clients, activity, started := app.h.Metrics("", true)
	database := "Unavailable"
	if app.library != nil && app.library.store.DB.PingContext(ctx) == nil {
		database = "Connected"
	}
	var size int64
	files := 0
	scanErr := filepath.WalkDir(app.cfg.DataDir, func(path string, d fs.DirEntry, err error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			return err
		}
		if d.Type().IsRegular() {
			info, e := d.Info()
			if e != nil {
				return e
			}
			size += info.Size()
			files++
		}
		return nil
	})
	storage := "Readable"
	if scanErr != nil {
		storage = "Incomplete scan"
	}
	cert := gin.H{"status": "Not observable", "detail": "TLS may terminate at a reverse proxy. No server certificate is configured."}
	if len(app.cfg.Certificate.Certificate) > 0 {
		cert = gin.H{"status": "Invalid certificate"}
		if leaf, err := x509.ParseCertificate(app.cfg.Certificate.Certificate[0]); err == nil {
			status := "Valid"
			if time.Now().Before(leaf.NotBefore) {
				status = "Not yet valid"
			} else if time.Now().After(leaf.NotAfter) {
				status = "Expired"
			} else if time.Until(leaf.NotAfter) < 30*24*time.Hour {
				status = "Expires soon"
			}
			cert = gin.H{"status": status, "expires": leaf.NotAfter, "detail": "Certificate loaded by this server; proxy certificates are separate."}
		}
	}
	backups := []gin.H{}
	users, userErr := app.userStorer.GetUsers()
	backupsAvailable := userErr == nil && app.library != nil
	if backupsAvailable {
		for _, user := range users {
			policy, e := app.library.store.Policy(common.SanitizeUid(user.ID))
			if e != nil {
				backupsAvailable = false
				continue
			}
			last, e := app.library.store.LatestRun(common.SanitizeUid(user.ID), "backup")
			if e != nil {
				backupsAvailable = false
				continue
			}
			backups = append(backups, gin.H{"user": user.ID, "enabled": policy.BackupEnabled, "schedule": policy.Schedule, "lastRun": last})
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"version": version,
		"compatibility": gin.H{
			"through": "3.27.1",
			"detail":  "Upstream documents file synchronization through 3.27.1. Newer firmware is untested; a minimum is not specified.",
			"source":  "https://github.com/ddvk/rmfakecloud#supported-devices",
		},
		"database":         database,
		"storage":          gin.H{"status": storage, "bytes": size, "files": files, "complete": scanErr == nil},
		"clients":          clients,
		"activity":         activity,
		"observedSince":    started,
		"certificate":      cert,
		"backups":          backups,
		"backupsAvailable": backupsAvailable,
		"checkedAt":        time.Now().UTC(),
	})
}
