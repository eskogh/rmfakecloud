package viewmodel

import "testing"

func TestValidateMove(t *testing.T) {
	tree := &DocumentTree{Entries: []Entry{
		&Directory{ID: "a", Entries: []Entry{&Directory{ID: "b", Entries: []Entry{&Document{ID: "c"}}}}},
		&Directory{ID: "d"},
		&Document{ID: "e"},
	}, Trash: []Entry{&Document{ID: "deleted"}}}
	for _, tc := range []struct {
		doc, parent string
		valid       bool
	}{
		{"a", "a", false}, {"a", "b", false}, {"a", "c", false},
		{"a", "e", false}, {"a", "missing", false}, {"missing", "d", false},
		{"a", "d", true}, {"c", "", true}, {"deleted", "d", true}, {"a", "trash", true},
	} {
		t.Run(tc.doc+"_to_"+tc.parent, func(t *testing.T) {
			err := ValidateMove(tree, tc.doc, tc.parent)
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%v, got error %v", tc.valid, err)
			}
		})
	}
}
