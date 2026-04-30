package main

import "testing"

func TestPluralSuffix(t *testing.T) {
	if pluralSuffix(1) != "" {
		t.Fatal("expected no suffix for singular")
	}
	if pluralSuffix(2) != "s" {
		t.Fatal("expected plural suffix for count>1")
	}
}
