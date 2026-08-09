package invite

import (
	"bufio"
	crand "crypto/rand"
	_ "embed"
	"fmt"
	"math/big"
	"strings"
	"sync"
)

// A recovery phrase is six words a person writes on paper. It replaces the
// eight-digit code for the same job and settles the question the digits never
// really answered.
//
// Eight digits is a hundred million, and all that stood between a stranger and
// an account was how well we counted their guesses - measured before any
// counting existed, thirty-two guesses went through in two seconds. Six words
// out of two thousand is seven times ten to the nineteenth: guessing stops
// being something to defend against and becomes something that cannot happen.
//
// The list is the BIP-39 English one, used by every hardware wallet and by
// Brave's sync: two thousand and forty-eight words, all lower-case ASCII, three
// to eight letters, and no two sharing their first four letters - so a
// half-remembered word is still unambiguous, and typos are visible rather than
// silent.
const phraseWords = 6

//go:embed wordlist.txt
var wordlistFile string

var (
	wordsOnce sync.Once
	words     []string
)

func wordlist() []string {
	wordsOnce.Do(func() {
		scanner := bufio.NewScanner(strings.NewReader(wordlistFile))
		for scanner.Scan() {
			if word := strings.TrimSpace(scanner.Text()); word != "" {
				words = append(words, word)
			}
		}
	})
	return words
}

// Phrase makes a recovery phrase: six words, chosen with crypto/rand, joined by
// single spaces.
func Phrase() (string, error) {
	list := wordlist()
	if len(list) < 1024 {
		// A short list means the file did not travel with the binary, and a
		// phrase drawn from a handful of words is worse than none.
		return "", fmt.Errorf("the word list holds %d words, which is not a word list", len(list))
	}

	chosen := make([]string, phraseWords)
	for i := range chosen {
		n, err := crand.Int(crand.Reader, big.NewInt(int64(len(list))))
		if err != nil {
			return "", fmt.Errorf("no randomness available: %w", err)
		}
		chosen[i] = list[n.Int64()]
	}

	return strings.Join(chosen, " "), nil
}

// NormalizePhrase is what both sides compare. A person types their paper back
// with a capital at the front, two spaces in the middle, a trailing one from
// the keyboard's autocomplete - none of that is a different phrase, and being
// strict about it would turn the one way back into a puzzle.
func NormalizePhrase(typed string) string {
	return strings.Join(strings.Fields(strings.ToLower(typed)), " ")
}
