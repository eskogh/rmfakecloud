package fs

import (
	"github.com/ddvk/rmfakecloud/internal/storage/models"
	"testing"
)

func TestDocumentChanges(t *testing.T) {
	doc := func(id, hash string) *models.HashDoc {
		return &models.HashDoc{HashEntry: models.HashEntry{EntryName: id, Hash: hash}, MetadataFile: models.MetadataFile{DocumentName: "Work"}}
	}
	before := &models.HashTree{Docs: []*models.HashDoc{doc("update", "old"), doc("delete", "old"), doc("same", "hash")}}
	after := &models.HashTree{Docs: []*models.HashDoc{doc("update", "new"), doc("create", "new"), doc("same", "hash")}}
	events := documentChanges("user", before, after)
	if len(events) != 3 {
		t.Fatal(events)
	}
	found := map[string]string{}
	for _, e := range events {
		found[e.Data.Document.ID] = e.Event
		if e.Data.User.ID != "user" || e.Data.Document.Name != "Work" {
			t.Fatal(e)
		}
	}
	if found["update"] != "document.updated" || found["create"] != "document.created" || found["delete"] != "document.deleted" {
		t.Fatal(found)
	}
}
