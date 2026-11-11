package syncstate

import "testing"

func index(changes []Change) map[string]Change {
	m := make(map[string]Change, len(changes))
	for _, c := range changes {
		key := string(c.Kind) + ":" + c.Path + "->" + c.OldPath
		m[key] = c
	}
	return m
}

func TestDiff_FirstRunAllAdded(t *testing.T) {
	changes := Diff(nil, map[string]string{"a": "1", "b": "2"})
	if len(changes) != 2 {
		t.Fatalf("beklenen 2, alınan %d: %v", len(changes), changes)
	}
	for _, c := range changes {
		if c.Kind != ChangeAdded {
			t.Errorf("beklenen added, alınan %s", c.Kind)
		}
	}
}

func TestDiff_NoChanges(t *testing.T) {
	changes := Diff(map[string]string{"a": "1"}, map[string]string{"a": "1"})
	if len(changes) != 0 {
		t.Fatalf("değişiklik beklenmiyordu: %v", changes)
	}
}

func TestDiff_Modified(t *testing.T) {
	changes := Diff(map[string]string{"a": "1", "b": "1"}, map[string]string{"a": "1", "b": "2"})
	if len(changes) != 1 {
		t.Fatalf("beklenen 1, alınan %d: %v", len(changes), changes)
	}
	if changes[0].Kind != ChangeModified || changes[0].Path != "b" {
		t.Errorf("yanlış değişiklik: %v", changes[0])
	}
}

func TestDiff_Deleted(t *testing.T) {
	changes := Diff(map[string]string{"a": "1", "b": "1"}, map[string]string{"a": "1"})
	if len(changes) != 1 {
		t.Fatalf("beklenen 1, alınan %d: %v", len(changes), changes)
	}
	if changes[0].Kind != ChangeDeleted || changes[0].OldPath != "b" {
		t.Errorf("yanlış değişiklik: %v", changes[0])
	}
}

func TestDiff_Renamed(t *testing.T) {
	changes := Diff(map[string]string{"old": "same"}, map[string]string{"new": "same"})
	if len(changes) != 1 {
		t.Fatalf("beklenen 1, alınan %d: %v", len(changes), changes)
	}
	c := changes[0]
	if c.Kind != ChangeRenamed || c.Path != "new" || c.OldPath != "old" {
		t.Errorf("yanlış rename: %v", c)
	}
}

func TestDiff_RenameAndUnrelatedDelete(t *testing.T) {
	// old -> new rename + x silinir; x rename'e karışmamalı.
	changes := Diff(
		map[string]string{"old": "h1", "x": "h2"},
		map[string]string{"new": "h1"},
	)
	idx := index(changes)
	if _, ok := idx["renamed:new->old"]; !ok {
		t.Errorf("rename tespit edilmedi: %v", changes)
	}
	if _, ok := idx["deleted:->x"]; !ok {
		t.Errorf("silme tespit edilmedi: %v", changes)
	}
	if len(changes) != 2 {
		t.Errorf("beklenen 2 değişiklik, alınan %d: %v", len(changes), changes)
	}
}

func TestDiff_RenameWithChangedContentIsDeletePlusAdd(t *testing.T) {
	changes := Diff(map[string]string{"old": "h1"}, map[string]string{"new": "h2"})
	idx := index(changes)
	if _, ok := idx["added:new->"]; !ok {
		t.Errorf("added bekleniyordu: %v", changes)
	}
	if _, ok := idx["deleted:->old"]; !ok {
		t.Errorf("deleted bekleniyordu: %v", changes)
	}
}
