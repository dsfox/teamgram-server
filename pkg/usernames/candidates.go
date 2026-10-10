// Package usernames gives every account a username of its own (#239).
//
// A person without one was mentioned in a group by whatever name the one
// mentioning them had saved - "Брат" instead of Eduard - because both clients
// fall back to a mention by name. With a username, picking anybody in the @
// suggestions inserts @username, which reads the same for everybody.
//
// A username given out here is for mentions, not a way in: a stranger cannot
// find the person by it (see Store.IsGenerated). Only one the person chose
// themselves is, as it was before (#181).
package usernames

import (
	"regexp"
	"strconv"
	"strings"
)

const (
	minLength = 5
	maxLength = 32
)

// The rules usernames follow on both clients and on the server's own check:
// a letter first, then letters, digits and underscores, five to thirty-two.
var shape = regexp.MustCompile(`^[a-z][a-z0-9_]{4,31}$`)

// Valid says whether name can be somebody's username.
func Valid(name string) bool {
	return shape.MatchString(name) && !strings.Contains(name, "__") && !strings.HasSuffix(name, "_")
}

// Steps are the names tried before any counter, in order: the first name, the
// first name and the first letter of the last name, the first name and the
// last name - written together, in Latin letters. A step that cannot be a
// username, being too short, is left out rather than padded.
func Steps(first, last string) []string {
	given, family := Latin(first), Latin(last)
	var steps []string
	add := func(name string) {
		name = fit(name)
		if !Valid(name) {
			return
		}
		for _, already := range steps {
			if already == name {
				return
			}
		}
		steps = append(steps, name)
	}
	add(given)
	if given != "" && family != "" {
		add(given + family[:1])
		add(given + family)
	}
	return steps
}

// Base is what a counter is put after once every step is taken: the last
// step's name, or "user" when the name gives nothing a username can start
// with - written in a script with no Latin letters, or beginning with a digit.
func Base(first, last string) string {
	given, family := Latin(first), Latin(last)
	base := given
	if given != "" {
		base = given + family
	}
	if base == "" || base[0] < 'a' || base[0] > 'z' {
		return "user"
	}
	return base
}

// Counted is base followed by n, cut to fit, or "" when that cannot be a
// username: "bob" with 1 to 9 is too short, so the first is "bob10".
func Counted(base string, n int) string {
	number := strconv.Itoa(n)
	if len(base)+len(number) > maxLength {
		base = base[:maxLength-len(number)]
	}
	name := base + number
	if !Valid(name) {
		return ""
	}
	return name
}

func fit(name string) string {
	if len(name) > maxLength {
		return name[:maxLength]
	}
	return name
}
