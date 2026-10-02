package types

import "testing"

func TestPersonalModelRefRoundTrip(t *testing.T) {
	ref := FormatPersonalModelRef("abc-1")
	if ref != "pm:abc-1" {
		t.Fatalf("got %q", ref)
	}
	id, ok := ParsePersonalModelRef(ref)
	if !ok || id != "abc-1" {
		t.Fatalf("parse: %q %v", id, ok)
	}
	if _, ok := ParsePersonalModelRef("workspace-model"); ok {
		t.Fatal("workspace id must not parse as personal")
	}
	if FormatPersonalModelRef("pm:x") != "pm:x" {
		t.Fatal("already-prefixed must stay")
	}
}
