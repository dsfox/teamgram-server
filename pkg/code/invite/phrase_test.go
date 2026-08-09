package invite

import (
	"strings"
	"testing"
)

// The list has to have travelled with the binary. A phrase drawn from whatever
// was left of a missing file would look fine and be worth nothing.
func TestTheWordListIsWhole(t *testing.T) {
	list := wordlist()

	if len(list) != 2048 {
		t.Fatalf("the list holds %d words, expected 2048", len(list))
	}

	seen := make(map[string]bool, len(list))
	prefixes := make(map[string]bool, len(list))
	for _, word := range list {
		if word != strings.ToLower(word) || strings.ContainsAny(word, " \t") {
			t.Fatalf("%q is not a plain lower-case word", word)
		}
		if seen[word] {
			t.Fatalf("%q appears twice, so the list is smaller than it looks", word)
		}
		seen[word] = true
		if len(word) >= 4 {
			// The point of the BIP-39 list: four letters name a word uniquely,
			// so a half-remembered one is still unambiguous.
			if prefixes[word[:4]] {
				t.Fatalf("%q shares its first four letters with another word", word)
			}
			prefixes[word[:4]] = true
		}
	}
}

func TestAPhraseIsSixDistinctlyRandomWords(t *testing.T) {
	first, err := Phrase()
	if err != nil {
		t.Fatal(err)
	}
	if got := len(strings.Fields(first)); got != phraseWords {
		t.Fatalf("the phrase is %d words, expected %d", got, phraseWords)
	}

	second, _ := Phrase()
	if first == second {
		t.Fatal("two phrases came out the same, so they are not random")
	}
}

// A person types their paper back with a capital at the front and a stray
// space at the end. That is the same phrase and must not be a wrong one.
func TestTypingItBackLooselyStillWorks(t *testing.T) {
	const written = "abandon ability able about above absent"

	for _, typed := range []string{
		"Abandon ability able about above absent",
		"  abandon   ability able  about above absent  ",
		"ABANDON ABILITY ABLE ABOUT ABOVE ABSENT",
		"abandon\tability able about above absent",
	} {
		if got := NormalizePhrase(typed); got != written {
			t.Errorf("%q read as %q, expected %q", typed, got, written)
		}
	}
}

// And a code from before phrases existed must pass through untouched, or every
// account that has one loses its way back.
func TestADigitCodeSurvivesNormalizing(t *testing.T) {
	if got := NormalizePhrase("48210773"); got != "48210773" {
		t.Errorf("a digit code became %q", got)
	}
}
