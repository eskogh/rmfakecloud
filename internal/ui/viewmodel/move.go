package viewmodel

import "fmt"

// ValidateMove checks the visible library before changing parent metadata.
// A client can be stale, so the UI's own checks are not sufficient here.
func ValidateMove(tree *DocumentTree, documentID, parentID string) error {
	parents := map[string]string{}
	folders := map[string]bool{}
	var walk func([]Entry, string)
	walk = func(entries []Entry, parent string) {
		for _, entry := range entries {
			switch e := entry.(type) {
			case *Directory:
				parents[e.ID] = parent
				folders[e.ID] = true
				walk(e.Entries, e.ID)
			case *Document:
				parents[e.ID] = parent
			}
		}
	}
	walk(tree.Entries, "")
	walk(tree.Trash, "trash")
	if _, exists := parents[documentID]; !exists {
		return fmt.Errorf("document not found")
	}
	if parentID != "" && parentID != "trash" && !folders[parentID] {
		return fmt.Errorf("destination folder not found")
	}
	seen := map[string]bool{}
	for current := parentID; current != "" && current != "trash"; current = parents[current] {
		if current == documentID || seen[current] {
			return fmt.Errorf("cannot move a folder into itself or its descendants")
		}
		seen[current] = true
	}
	return nil
}
