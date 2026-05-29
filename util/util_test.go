package util

import "testing"

func TestPluralSuffix(t *testing.T) {
	if PluralSuffix(1) != "" {
		t.Fatal("expected no suffix for singular")
	}
	if PluralSuffix(2) != "s" {
		t.Fatal("expected plural suffix for count>1")
	}
}
