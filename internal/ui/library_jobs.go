package ui

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/ddvk/rmfakecloud/internal/common"
	"github.com/ddvk/rmfakecloud/internal/integrations"
	"github.com/ddvk/rmfakecloud/internal/library"
	"github.com/ddvk/rmfakecloud/internal/ui/viewmodel"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
)

type libraryService struct {
	store   *library.Store
	app     *ReactAppWrapper
	ctx     context.Context
	cancel  context.CancelFunc
	mu      sync.Mutex
	running map[string]bool
	wg      sync.WaitGroup
	slots   chan struct{}
}
type libraryDoc struct{ ID, Name, Parent, Version string }

func (app *ReactAppWrapper) startLibrary() {
	store, err := library.Open(filepath.Join(app.cfg.DataDir, "library"))
	if err != nil {
		log.Errorf("Library features unavailable: %v", err)
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	service := &libraryService{store: store, app: app, ctx: ctx, cancel: cancel, running: map[string]bool{}, slots: make(chan struct{}, 2)}
	app.library = service
	service.wg.Add(1)
	go func() {
		defer service.wg.Done()
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case now := <-ticker.C:
				service.schedule(now.UTC())
			}
		}
	}()
}
func (app *ReactAppWrapper) CloseLibrary() {
	if app.library != nil {
		app.library.cancel()
		app.library.wg.Wait()
		app.library.store.DB.Close()
	}
}
func (s *libraryService) backend(uid string) (backend, error) {
	user, err := s.app.userStorer.GetUser(uid)
	if err != nil {
		return nil, err
	}
	version := common.Sync10
	if user.Sync15 {
		version = common.Sync15
	}
	return s.app.backends[version], nil
}
func libraryDocuments(tree *viewmodel.DocumentTree) []libraryDoc {
	result := []libraryDoc{}
	var visit func([]viewmodel.Entry, string)
	visit = func(entries []viewmodel.Entry, parent string) {
		for _, entry := range entries {
			switch d := entry.(type) {
			case *viewmodel.Directory:
				visit(d.Entries, d.ID)
			case *viewmodel.Document:
				result = append(result, libraryDoc{ID: d.ID, Name: d.Name, Parent: parent, Version: fmt.Sprintf("%s:%d:%s", d.LastModified.UTC().Format(time.RFC3339Nano), d.Size, d.Name)})
			}
		}
	}
	visit(tree.Entries, "")
	return result
}
func (s *libraryService) inventory(uid string, b backend) ([]libraryDoc, error) {
	tree, err := b.GetDocumentTree(uid)
	if err != nil {
		return nil, err
	}
	docs := libraryDocuments(tree)
	if modern, ok := b.(*backend15); ok {
		hashTree, err := modern.blobHandler.GetCachedTree(uid)
		if err != nil {
			return nil, err
		}
		versions := map[string]string{}
		for _, doc := range hashTree.Docs {
			versions[doc.EntryName] = doc.Hash
		}
		for i := range docs {
			docs[i].Version = versions[docs[i].ID]
		}
	}
	return docs, nil
}
func (s *libraryService) start(uid, kind, onlyDoc string) error {
	if err := library.ValidateKind(kind); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ctx.Err() != nil {
		return fmt.Errorf("server is stopping")
	}
	if s.running[uid] {
		return fmt.Errorf("a library job is already running for this account")
	}
	select {
	case s.slots <- struct{}{}:
	default:
		return fmt.Errorf("library workers are busy; try again shortly")
	}
	id, err := s.store.StartRun(uid, kind)
	if err != nil {
		<-s.slots
		return err
	}
	s.running[uid] = true
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer func() { s.mu.Lock(); delete(s.running, uid); s.mu.Unlock(); <-s.slots }()
		completed, failures := s.execute(uid, kind, onlyDoc)
		if err := s.store.FinishRun(id, completed, failures); err != nil {
			log.Error("Could not save library job result: ", err)
		}
	}()
	return nil
}
func (s *libraryService) execute(uid, kind, onlyDoc string) (int, []string) {
	b, err := s.backend(uid)
	if err != nil {
		return 0, []string{err.Error()}
	}
	docs, err := s.inventory(uid, b)
	if err != nil {
		return 0, []string{err.Error()}
	}
	policy, err := s.store.Policy(uid)
	if err != nil {
		return 0, []string{err.Error()}
	}
	done := 0
	failures := []string{}
	live := map[string]bool{}
	for _, doc := range docs {
		live[doc.ID] = true
		if onlyDoc != "" && doc.ID != onlyDoc {
			continue
		}
		if s.ctx.Err() != nil {
			failures = append(failures, "Job interrupted by server shutdown")
			break
		}
		switch kind {
		case "index":
			err = s.index(uid, b, doc)
		case "snapshots":
			err = s.snapshot(uid, b, doc, policy.Retain)
		case "backup":
			err = s.backup(uid, b, doc, policy)
		}
		if err != nil {
			failures = append(failures, doc.Name+": "+err.Error())
		} else {
			done++
		}
	}
	if onlyDoc != "" && !live[onlyDoc] {
		failures = append(failures, "Document no longer exists")
	}
	if kind == "index" && onlyDoc == "" {
		if err := s.store.PruneIndex(uid, live); err != nil {
			failures = append(failures, err.Error())
		}
	}
	return done, failures
}
func exportFile(b backend, uid, id, format, destination string) (int64, error) {
	stream, err := b.Export(uid, id, format, 0)
	if err != nil {
		return 0, err
	}
	defer stream.Close()
	return library.WriteFile(destination, stream)
}
func (s *libraryService) index(uid string, b backend, doc libraryDoc) error {
	// Include the engine URL in the fingerprint so configuring OCR reindexes PDFs.
	endpoint := os.Getenv("LIBRARY_OCR_URL")
	version := doc.Version + ":" + endpoint
	if s.store.IndexedVersion(uid, doc.ID) == version {
		return nil
	}
	dir, err := os.MkdirTemp("", "rm-index-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	filename := filepath.Join(dir, "document.pdf")
	if _, err = exportFile(b, uid, doc.ID, "pdf", filename); err != nil {
		return err
	}
	pages, err := library.ExtractPDF(filename)
	if err != nil {
		return err
	}
	if endpoint != "" {
		recognized, err := library.OCR(s.ctx, endpoint, filename)
		if err != nil {
			return fmt.Errorf("OCR failed: %w", err)
		}
		pages = library.MergePages(pages, recognized)
	}
	return s.store.Index(uid, doc.ID, version, pages)
}
func (s *libraryService) snapshot(uid string, b backend, doc libraryDoc, retain int) error {
	previous, err := s.store.Snapshots(uid, doc.ID)
	if err != nil {
		return err
	}
	if len(previous) > 0 && previous[0].Version == doc.Version {
		return nil
	}
	id := uuid.NewString()
	rawPath := s.store.SnapshotPath(uid, id, "rmdoc")
	pdfPath := s.store.SnapshotPath(uid, id, "pdf")
	size, err := exportFile(b, uid, doc.ID, "rmdoc", rawPath)
	if err != nil {
		return err
	}
	_, pdfErr := exportFile(b, uid, doc.ID, "pdf", pdfPath)
	// Never associate exports with a version that changed while they were produced.
	current, err := s.inventory(uid, b)
	if err != nil {
		os.Remove(rawPath)
		os.Remove(pdfPath)
		return err
	}
	stable := false
	for _, d := range current {
		if d.ID == doc.ID && d.Version == doc.Version {
			stable = true
		}
	}
	if !stable {
		os.Remove(rawPath)
		os.Remove(pdfPath)
		return fmt.Errorf("document changed during capture; retry")
	}
	snapshot := library.Snapshot{ID: id, DocumentID: doc.ID, Name: doc.Name, Version: doc.Version, Created: time.Now().UTC().Format(time.RFC3339Nano), Size: size, HasPDF: pdfErr == nil}
	if err = s.store.AddSnapshot(uid, snapshot); err != nil {
		os.Remove(rawPath)
		os.Remove(pdfPath)
		return err
	}
	for _, old := range append([]library.Snapshot{snapshot}, previous...)[min(retain, len(previous)+1):] {
		if err = s.store.DeleteSnapshot(uid, old.ID); err != nil {
			return err
		}
		os.Remove(s.store.SnapshotPath(uid, old.ID, "rmdoc"))
		os.Remove(s.store.SnapshotPath(uid, old.ID, "pdf"))
	}
	return nil
}
func (s *libraryService) backup(uid string, b backend, doc libraryDoc, p library.Policy) error {
	destination := p.Destination + ":" + p.Folder + ":" + p.Format
	// Daily exports are incremental; weekly runs export the whole current library.
	if p.Schedule == "daily" && s.store.Exported(uid, destination, doc.ID, doc.Version) {
		return nil
	}
	basename := common.Sanitize(doc.Name)
	if len(basename) > 100 {
		basename = basename[:100]
	}
	basename = basename + "-" + doc.ID + "-" + time.Now().UTC().Format("20060102T150405Z")
	stream, err := b.Export(uid, doc.ID, p.Format, 0)
	if err != nil {
		return err
	}
	defer stream.Close()
	if p.Destination == "local" {
		// Web clients cannot choose arbitrary host paths; NAS mounts can be configured by an administrator.
		root := os.Getenv("LIBRARY_BACKUP_DIR")
		if root == "" {
			root = filepath.Join(s.store.Root, "backups")
		}
		_, err = library.WriteFile(filepath.Join(root, library.AccountKey(uid), basename+"."+p.Format), stream)
	} else {
		provider, e := integrations.GetStorageIntegrationProvider(s.app.userStorer, uid, p.Destination)
		if e != nil {
			return e
		}
		folder := "root"
		if p.Folder != "" {
			folder = base64.URLEncoding.EncodeToString([]byte(p.Folder))
		}
		_, err = provider.Upload(folder, basename, p.Format, stream)
	}
	if err != nil {
		return err
	}
	return s.store.MarkExported(uid, destination, doc.ID, doc.Version)
}
func backupDue(p library.Policy, runs []library.Run, now time.Time) bool {
	boundary := time.Date(now.Year(), now.Month(), now.Day(), 2, 0, 0, 0, time.UTC)
	if p.Schedule == "weekly" {
		boundary = boundary.AddDate(0, 0, -int(boundary.Weekday()))
	}
	if now.Before(boundary) {
		return false
	}
	for _, r := range runs {
		if r.Kind == "backup" {
			started, err := time.Parse(time.RFC3339, r.Started)
			if err == nil && !started.Before(boundary) {
				return false
			}
		}
	}
	return true
}
func (s *libraryService) schedule(now time.Time) {
	users, err := s.app.userStorer.GetUsers()
	if err != nil {
		log.Warn("Library scheduler: ", err)
		return
	}
	for _, user := range users {
		uid := common.SanitizeUid(user.ID)
		p, err := s.store.Policy(uid)
		if err != nil {
			continue
		}
		runs := []library.Run{{Kind: "backup", Started: s.store.LastStarted(uid, "backup")}}
		if p.BackupEnabled && backupDue(p, runs, now) {
			_ = s.start(uid, "backup", "")
			continue
		}
		last := func(kind string) time.Time {
			t, _ := time.Parse(time.RFC3339, s.store.LastStarted(uid, kind))
			return t
		}
		if p.AutoIndex && now.Sub(last("index")) >= 15*time.Minute {
			_ = s.start(uid, "index", "")
			continue
		}
		if p.Snapshots && now.Sub(last("snapshots")) >= time.Minute {
			_ = s.start(uid, "snapshots", "")
		}
	}
}
