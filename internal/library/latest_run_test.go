package library

import "testing"

func TestLatestBackupSurvivesRecentIndexJobs(t *testing.T) {
	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.DB.Close()
	id, err := store.StartRun("alice", "backup")
	if err != nil {
		t.Fatal(err)
	}
	if err = store.FinishRun(id, 12, nil); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 35; i++ {
		if _, err = store.StartRun("alice", "index"); err != nil {
			t.Fatal(err)
		}
	}
	run, err := store.LatestRun("alice", "backup")
	if err != nil || run == nil || run.ID != id || run.Completed != 12 {
		t.Fatalf("missing backup: %v %v", run, err)
	}
	run, err = store.LatestRun("bob", "backup")
	if err != nil || run != nil {
		t.Fatalf("account isolation failed: %v %v", run, err)
	}
}
