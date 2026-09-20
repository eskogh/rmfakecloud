package fs

import (
	"context"

	"github.com/ddvk/rmfakecloud/internal/automation"
	"github.com/ddvk/rmfakecloud/internal/storage/models"
)

type committedRoot struct {
	*LocalBlobStorage
	hash string
}

func (r committedRoot) GetRootIndex() (string, int64, error) { return r.hash, 0, nil }

// HandleEvent reads immutable committed roots on a background worker. Raw blob
// uploads never emit document events, and automation failures cannot fail sync.
func (fs *FileSystemStorage) HandleEvent(ctx context.Context, e automation.Event) error {
	if e.Event != "internal.root_committed" || e.Data.Commit == nil {
		return nil
	}
	before, after := &models.HashTree{}, &models.HashTree{}
	source := fs.BlobStorage(e.Data.User.ID)
	if _, err := before.Mirror(committedRoot{source, e.Data.Commit.Before}); err != nil {
		return err
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if _, err := after.Mirror(committedRoot{source, e.Data.Commit.After}); err != nil {
		return err
	}
	for _, event := range documentChanges(e.Data.User.ID, before, after) {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		fs.Cfg.PublishEvent(event)
	}
	return nil
}
func documentChanges(uid string, before, after *models.HashTree) []automation.Event {
	old := make(map[string]*models.HashDoc, len(before.Docs))
	for _, doc := range before.Docs {
		old[doc.EntryName] = doc
	}
	var events []automation.Event
	add := func(name string, doc *models.HashDoc) {
		event := automation.NewEvent(name, uid)
		event.Data.Source = "sync15"
		event.Data.Document = &automation.Document{ID: doc.EntryName, Name: doc.DocumentName, Type: string(doc.CollectionType)}
		events = append(events, event)
	}
	for _, doc := range after.Docs {
		previous, exists := old[doc.EntryName]
		if !exists {
			add("document.created", doc)
		} else if previous.Hash != doc.Hash {
			add("document.updated", doc)
		}
		delete(old, doc.EntryName)
	}
	for _, doc := range old {
		add("document.deleted", doc)
	}
	return events
}
