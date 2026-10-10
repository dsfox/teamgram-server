package usernames

import (
	"reflect"
	"strings"
	"testing"
)

func TestLatin(t *testing.T) {
	for name, want := range map[string]string{
		"Эдуард":     "eduard",
		"Евгений":    "evgeniy",
		"Ольга":      "olga",
		"Балягина":   "balyagina",
		"Щукин":      "shchukin",
		"Хохлова":    "khokhlova",
		"VOVA":       "vova",
		"Анна-Мария": "annamariya",
		"Ёжик":       "ezhik",
		"Їжак":       "yizhak",
		"José":       "jos",
		"王":          "",
		"R2 D2":      "r2d2",
		"Ice_9":      "ice9",
		"  Alice  ":  "alice",
	} {
		if got := Latin(name); got != want {
			t.Errorf("Latin(%q) = %q, want %q", name, got, want)
		}
	}
}

func TestSteps(t *testing.T) {
	for _, c := range []struct {
		first, last string
		want        []string
	}{
		// The examples agreed with the owner on 10 October.
		{"Эдуард", "Голубничий", []string{"eduard", "eduardg", "eduardgolubnichiy"}},
		{"Ольга", "Балягина", []string{"olgab", "olgabalyagina"}},
		{"Евгений", "Балягин", []string{"evgeniy", "evgeniyb", "evgeniybalyagin"}},
		{"VOVA", "BVA", []string{"vovab", "vovabva"}},
		{"Номад", "", []string{"nomad"}},
		// Too short at every step: only a counter makes it a username.
		{"Bob", "", nil},
		// Short parts that add up: the last step is long enough.
		{"Eve", "Li", []string{"eveli"}},
		// Nothing a username can start with.
		{"王", "", nil},
		{"2pac", "", nil},
	} {
		if got := Steps(c.first, c.last); !reflect.DeepEqual(got, c.want) {
			t.Errorf("Steps(%q, %q) = %q, want %q", c.first, c.last, got, c.want)
		}
	}
}

func TestBaseAndCounter(t *testing.T) {
	for _, c := range []struct {
		first, last string
		n           int
		want        string
	}{
		{"Эдуард", "Голубничий", 1, "eduardgolubnichiy1"},
		{"Alice", "", 2, "alice2"},
		{"Bob", "", 1, ""},
		{"Bob", "", 9, ""},
		{"Bob", "", 10, "bob10"},
		{"王", "", 1, "user1"},
		{"2pac", "", 3, "user3"},
	} {
		if got := Counted(Base(c.first, c.last), c.n); got != c.want {
			t.Errorf("Counted(Base(%q, %q), %d) = %q, want %q", c.first, c.last, c.n, got, c.want)
		}
	}
}

func TestEverythingGivenOutIsAUsername(t *testing.T) {
	long := strings.Repeat("Длинное", 10)
	for _, name := range [][2]string{
		{long, long}, {"Эдуард", "Голубничий"}, {"Bob", ""}, {"A", "B"}, {"王", "李"},
	} {
		for _, step := range Steps(name[0], name[1]) {
			if !Valid(step) || len(step) > maxLength {
				t.Errorf("step %q for %q is not a username", step, name)
			}
		}
		for n := 1; n < 1200; n++ {
			if counted := Counted(Base(name[0], name[1]), n); counted != "" && (!Valid(counted) || len(counted) > maxLength) {
				t.Errorf("counted %q for %q is not a username", counted, name)
			}
		}
	}
	if got := Counted(Base(long, long), 123); len(got) != maxLength || !strings.HasSuffix(got, "123") {
		t.Errorf("a long name is cut to fit its counter: %q", got)
	}
}

func TestValid(t *testing.T) {
	for name, want := range map[string]bool{
		"alice": true, "bob10": true, "a_b_c": true,
		"bob": false, "1alice": false, "alice_": false, "al__ce": false,
		"Alice": false, strings.Repeat("a", 32): true, strings.Repeat("a", 33): false,
	} {
		if got := Valid(name); got != want {
			t.Errorf("Valid(%q) = %v, want %v", name, got, want)
		}
	}
}
