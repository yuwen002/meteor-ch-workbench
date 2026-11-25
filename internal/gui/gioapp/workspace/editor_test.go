package workspace

import "testing"

func TestEditorStateTracksDirtyAgainstSavedBaseline(t *testing.T) {
	var editor EditorState
	if editor.Dirty() {
		t.Fatal("new editor must be clean")
	}
	editor.SetText("SELECT 1")
	if !editor.Dirty() {
		t.Fatal("edited editor must be dirty")
	}
	if got := editor.Text(); got != "SELECT 1" {
		t.Fatalf("text = %q", got)
	}
	editor.MarkSaved()
	if editor.Dirty() {
		t.Fatal("saved editor must be clean")
	}
	editor.SetText("SELECT 2")
	if !editor.Dirty() {
		t.Fatal("editing after save must be dirty")
	}
	editor.SetText("SELECT 1")
	if editor.Dirty() {
		t.Fatal("restoring saved text must be clean")
	}
}

func TestEditorStateResetClearsTextAndDirtyState(t *testing.T) {
	var editor EditorState
	editor.SetText("SELECT 1")
	editor.Reset()
	if editor.Text() != "" || editor.Dirty() {
		t.Fatalf("reset editor = text %q, dirty %v", editor.Text(), editor.Dirty())
	}
}
