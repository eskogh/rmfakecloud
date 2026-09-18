package ui

import (
	"net/http"
	"os"
	"strings"

	"github.com/ddvk/rmfakecloud/internal/common"
	"github.com/ddvk/rmfakecloud/internal/integrations"
	"github.com/ddvk/rmfakecloud/internal/library"
	"github.com/ddvk/rmfakecloud/internal/ui/viewmodel"
	"github.com/gin-gonic/gin"
)

func (app *ReactAppWrapper) libraryAvailable(c *gin.Context) bool {
	if app.library == nil {
		c.AbortWithStatusJSON(http.StatusServiceUnavailable, viewmodel.NewErrorResponse("Library database is unavailable"))
		return false
	}
	return true
}
func (app *ReactAppWrapper) libraryState(c *gin.Context) {
	if !app.libraryAvailable(c) {
		return
	}
	uid := userID(c)
	store := app.library.store
	metadata, err := store.Metadata(uid)
	if err != nil {
		badReq(c, err.Error())
		return
	}
	policy, err := store.Policy(uid)
	if err != nil {
		badReq(c, err.Error())
		return
	}
	runs, err := store.Runs(uid)
	if err != nil {
		badReq(c, err.Error())
		return
	}
	destinations := []gin.H{{"id": "local", "name": "Server filesystem / NAS mount"}}
	user, err := app.userStorer.GetUser(uid)
	if err != nil {
		badReq(c, err.Error())
		return
	}
	for _, integration := range user.Integrations {
		switch integration.Provider {
		case integrations.WebdavProvider, integrations.LocalfsProvider, integrations.DropboxProvider, integrations.FtpProvider:
			destinations = append(destinations, gin.H{"id": integration.ID, "name": integration.Name})
		}
	}
	var indexed int
	_ = store.DB.QueryRow("SELECT count(*) FROM indexed WHERE uid=?", uid).Scan(&indexed)
	c.JSON(http.StatusOK, gin.H{"metadata": metadata, "policy": policy, "runs": runs, "destinations": destinations, "ocrConfigured": os.Getenv("LIBRARY_OCR_URL") != "", "restoreSupported": user.Sync15, "indexedDocuments": indexed})
}
func findLibraryEntry(entries []viewmodel.Entry, id string) bool {
	for _, entry := range entries {
		switch item := entry.(type) {
		case *viewmodel.Document:
			if item.ID == id {
				return true
			}
		case *viewmodel.Directory:
			if item.ID == id || findLibraryEntry(item.Entries, id) {
				return true
			}
		}
	}
	return false
}
func (app *ReactAppWrapper) setLibraryMetadata(c *gin.Context) {
	if !app.libraryAvailable(c) {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 16<<10)
	var metadata library.Metadata
	if err := c.ShouldBindJSON(&metadata); err != nil {
		badReq(c, "Invalid metadata")
		return
	}
	id := common.ParamS(docIDParam, c)
	tree, err := app.getBackend(c).GetDocumentTree(userID(c))
	if err != nil {
		badReq(c, err.Error())
		return
	}
	if !findLibraryEntry(tree.Entries, id) && !findLibraryEntry(tree.Trash, id) {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	if err = app.library.store.SetMetadata(userID(c), id, metadata); err != nil {
		badReq(c, err.Error())
		return
	}
	c.Status(http.StatusNoContent)
}
func (app *ReactAppWrapper) librarySearch(c *gin.Context) {
	if !app.libraryAvailable(c) {
		return
	}
	hits, err := app.library.store.Search(userID(c), c.Query("q"))
	if err != nil {
		badReq(c, err.Error())
		return
	}
	tree, err := app.getBackend(c).GetDocumentTree(userID(c))
	if err != nil {
		badReq(c, err.Error())
		return
	}
	live := map[string]libraryDoc{}
	for _, doc := range libraryDocuments(tree) {
		live[doc.ID] = doc
	}
	result := []gin.H{}
	for _, hit := range hits {
		if doc, ok := live[hit.DocumentID]; ok {
			result = append(result, gin.H{"documentId": hit.DocumentID, "name": doc.Name, "page": hit.Page, "snippet": hit.Snippet, "source": hit.Source})
		}
	}
	c.JSON(http.StatusOK, result)
}
func (app *ReactAppWrapper) saveLibraryPolicy(c *gin.Context) {
	if !app.libraryAvailable(c) {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 16<<10)
	var policy library.Policy
	if err := c.ShouldBindJSON(&policy); err != nil {
		badReq(c, "Invalid library policy")
		return
	}
	if policy.Destination != "local" {
		if _, err := integrations.GetStorageIntegrationProvider(app.userStorer, userID(c), policy.Destination); err != nil {
			badReq(c, "Destination is not one of your storage integrations")
			return
		}
	}
	if err := app.library.store.SetPolicy(userID(c), policy); err != nil {
		badReq(c, err.Error())
		return
	}
	c.Status(http.StatusNoContent)
}
func (app *ReactAppWrapper) runLibraryJob(c *gin.Context) {
	if !app.libraryAvailable(c) {
		return
	}
	kind := c.Param("kind")
	if err := library.ValidateKind(kind); err != nil {
		badReq(c, err.Error())
		return
	}
	if err := app.library.start(userID(c), kind, c.Query("documentId")); err != nil {
		c.AbortWithStatusJSON(http.StatusConflict, viewmodel.NewErrorResponse(err.Error()))
		return
	}
	c.Status(http.StatusAccepted)
}
func (app *ReactAppWrapper) listSnapshots(c *gin.Context) {
	if !app.libraryAvailable(c) {
		return
	}
	snapshots, err := app.library.store.Snapshots(userID(c), common.ParamS(docIDParam, c))
	if err != nil {
		badReq(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, snapshots)
}
func (app *ReactAppWrapper) downloadSnapshot(c *gin.Context) {
	if !app.libraryAvailable(c) {
		return
	}
	snapshot, err := app.library.store.Snapshot(userID(c), c.Param("snapshotid"))
	if err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	format := c.DefaultQuery("type", "rmdoc")
	if format != "rmdoc" && format != "pdf" {
		badReq(c, "Unknown snapshot format")
		return
	}
	if format == "pdf" && !snapshot.HasPDF {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	c.Header("Cache-Control", "private, no-store")
	if format == "pdf" {
		c.Header("Content-Type", "application/pdf")
	} else {
		c.Header("Content-Disposition", `attachment; filename="`+snapshot.ID+`.rmdoc"`)
	}
	c.File(app.library.store.SnapshotPath(userID(c), snapshot.ID, format))
}
func (app *ReactAppWrapper) restoreSnapshot(c *gin.Context) {
	if !app.libraryAvailable(c) {
		return
	}
	uid := userID(c)
	user, err := app.userStorer.GetUser(uid)
	if err != nil {
		badReq(c, err.Error())
		return
	}
	if !user.Sync15 {
		badReq(c, "Restoring a copy requires sync 1.5")
		return
	}
	snapshot, err := app.library.store.Snapshot(uid, c.Param("snapshotid"))
	if err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	b := app.getBackend(c)
	docs, err := app.library.inventory(uid, b)
	if err != nil {
		badReq(c, err.Error())
		return
	}
	parent := ""
	for _, doc := range docs {
		if doc.ID == snapshot.DocumentID {
			parent = doc.Parent
		}
	}
	restored, err := library.CloneArchive(app.library.store.SnapshotPath(uid, snapshot.ID, "rmdoc"), strings.TrimSpace(snapshot.Name)+" (restored)", parent)
	if err != nil {
		badReq(c, err.Error())
		return
	}
	defer restored.Close()
	defer os.Remove(restored.Name())
	doc, err := b.CreateDocument(uid, "restored.rmdoc", parent, restored)
	if err != nil {
		badReq(c, err.Error())
		return
	}
	b.Sync(uid)
	c.JSON(http.StatusCreated, gin.H{"documentId": doc.ID})
}
