package core

import (
	"strings"
	"testing"
	"unicode/utf16"
)

func TestTheSignUpTermsLinkWhereTheyPoint(t *testing.T) {
	terms := SignUpTerms()
	if !strings.Contains(terms.Text, "zero tolerance") || !strings.Contains(terms.Text, "не терпим") {
		t.Fatalf("the terms say nothing about what is not tolerated, in one language or both: %q", terms.Text)
	}

	// The link is counted the way a client counts it, in UTF-16 units: what
	// it covers must be the address and nothing else, in both halves.
	units := utf16.Encode([]rune(terms.Text))
	if len(terms.Entities) != 2 {
		t.Fatalf("want a link in each language, got %d", len(terms.Entities))
	}
	for _, entity := range terms.Entities {
		covered := string(utf16.Decode(units[entity.Offset : entity.Offset+entity.Length]))
		if covered != termsURL {
			t.Errorf("a link at %d covers %q, not the terms address", entity.Offset, covered)
		}
	}
}
