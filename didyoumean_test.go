// Copyright (c) 2026, the go-ruby-did-you-mean/did-you-mean authors
//
// SPDX-License-Identifier: BSD-3-Clause

package didyoumean

import (
	"reflect"
	"testing"
)

// rubyMethods is the enumerable-method dictionary used in several cases; it
// mirrors the kind of method list did_you_mean suggests over in real Ruby.
var rubyMethods = []string{
	"map", "select", "reject", "collect", "inject", "each",
	"detect", "find_all", "flatten", "compact",
}

// correctCases are golden vectors captured from MRI 4.0.5's
// DidYouMean::SpellChecker#correct. They are deterministic and ruby-free, so
// they alone drive the 100% coverage gate on the Windows and qemu lanes where
// the oracle skips. The oracle test re-derives the same expectations live.
var correctCases = []struct {
	name  string
	input string
	dict  []string
	want  []string
}{
	// Single-substitution mistype: one ranked suggestion.
	{"mistype_simple", "collekt", rubyMethods, []string{"collect"}},
	{"mistype_reject", "rerject", rubyMethods, []string{"reject"}},
	// Adjacent transposition still resolves to the intended method.
	{"transpose", "recjet", rubyMethods, []string{"reject"}},
	{"flatten", "flaten", rubyMethods, []string{"flatten"}},
	// Case folds away (normalize downcases): "FNid" matches "find".
	{"case_upper", "FNid", []string{"find"}, []string{"find"}},
	// No candidate clears the threshold → empty.
	{"empty_result", "xyz", rubyMethods, nil},
	// Empty input → empty (length 0, every distance fails the threshold).
	{"empty_input", "", rubyMethods, nil},
	// The exact (un-normalized) input is rejected from its own suggestions.
	{"exact_rejected", "map", rubyMethods, nil},
	// "@" sigils are stripped before comparison; only the bare name survives the
	// half-length comparison MRI applies here.
	{"ivar_strip", "@ivar", []string{"@ivar", "@other_ivar", "ivar"}, []string{"ivar"}},
	// Short input (≤3 chars) uses the 0.77 threshold and ranks ties by JW.
	{"short_thresh", "lst", []string{"first", "last", "least", "lost"}, []string{"lost", "last"}},
	{"misspell_two", "lst", []string{"list", "last", "cost"}, []string{"last", "list"}},
	{"multi_result", "zlast", []string{"first", "last", "least", "lost"}, []string{"last", "least"}},
	// Long input (>3 chars) uses the 0.834 threshold.
	{"long_word", "configurtion", []string{"configuration", "configure", "config"}, []string{"configuration"}},
	// Misspell fallback: nothing clears the Levenshtein mistype filter, so the
	// first candidate within half the shorter length is returned (captured from
	// MRI via a randomized search for the corrections.empty? branch).
	{"misspell_fallback", "bij", []string{"bicc"}, []string{"bicc"}},
	{"fallback_transpose", "ypgk", []string{"pyigk"}, []string{"pyigk"}},
	// JaroWinkler clears the threshold but the word is too long for the
	// half-length misspell test → empty (the corrections.empty? + reject path).
	{"fallback_too_long", "zab", []string{"jzyabh"}, nil},
	// Misspell fallback where the candidate is shorter than the input, so the
	// half-length test uses the candidate's length (the wl < length branch); the
	// Levenshtein distance still exceeds it, so the result is empty.
	{"fallback_shorter_candidate", "eqgvpa", []string{"eqv"}, nil},
}

func TestCorrect(t *testing.T) {
	for _, c := range correctCases {
		t.Run(c.name, func(t *testing.T) {
			got := Correct(c.input, c.dict)
			if len(got) == 0 && len(c.want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("Correct(%q, %v) = %#v, want %#v", c.input, c.dict, got, c.want)
			}
		})
	}
}

// TestCorrectEmptyDictionary covers the no-candidate path with an empty dict.
func TestCorrectEmptyDictionary(t *testing.T) {
	if got := Correct("anything", nil); len(got) != 0 {
		t.Errorf("Correct over empty dict = %#v, want empty", got)
	}
}

func TestLevenshteinDistance(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"", "", 0},
		{"", "abc", 3}, // n == 0 branch
		{"abc", "", 3}, // m == 0 branch
		{"abc", "abc", 0},
		{"kitten", "sitting", 3},
		{"flaw", "lawn", 2},
		{"collekt", "collect", 1},
		{"café", "cafe", 1}, // multibyte code points
	}
	for _, c := range cases {
		if got := levenshteinDistance(c.a, c.b); got != c.want {
			t.Errorf("levenshteinDistance(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestMin3(t *testing.T) {
	cases := []struct {
		a, b, c, want int
	}{
		{1, 2, 3, 1}, // a smallest
		{3, 1, 2, 1}, // b smallest
		{3, 2, 1, 1}, // c smallest
		{2, 2, 3, 2}, // a == b, b < c
	}
	for _, tc := range cases {
		if got := min3(tc.a, tc.b, tc.c); got != tc.want {
			t.Errorf("min3(%d,%d,%d) = %d, want %d", tc.a, tc.b, tc.c, got, tc.want)
		}
	}
}

func TestJaroDistance(t *testing.T) {
	const eps = 1e-12
	cases := []struct {
		a, b string
		want float64
	}{
		{"", "", 0},       // m == 0 branch
		{"abc", "xyz", 0}, // no matches → m == 0
		{"abc", "abc", 1}, // length2 <= 3 → range 0
		{"martha", "marhta", 0.9444444444444445},
		{"dwayne", "duane", 0.8222222222222223},
		{"dixon", "dicksonx", 0.7666666666666666},
	}
	for _, c := range cases {
		got := jaroDistance(c.a, c.b)
		if got-c.want > eps || c.want-got > eps {
			t.Errorf("jaroDistance(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

func TestJaroWinklerDistance(t *testing.T) {
	const eps = 1e-12
	cases := []struct {
		a, b string
		want float64
	}{
		// jaro <= threshold → returned unchanged (Jaro of these is 0).
		{"abc", "xyz", 0},
		// Common prefix bonus applied.
		{"martha", "marhta", 0.9611111111111111},
		{"dwayne", "duane", 0.8400000000000001},
		{"dixon", "dicksonx", 0.8133333333333332},
		// Prefix cap at 4: a long shared prefix stops contributing after 4.
		{"abcdefg", "abcdxyz", 0.8285714285714286},
	}
	for _, c := range cases {
		got := jaroWinklerDistance(c.a, c.b)
		if got-c.want > eps || c.want-got > eps {
			t.Errorf("jaroWinklerDistance(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

func TestNormalize(t *testing.T) {
	cases := []struct{ in, want string }{
		{"Map", "map"},
		{"@ivar", "ivar"},
		{"@@cvar", "cvar"},
		{"FNid", "fnid"},
	}
	for _, c := range cases {
		if got := normalize(c.in); got != c.want {
			t.Errorf("normalize(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestCeilQuarter(t *testing.T) {
	cases := []struct{ n, want int }{
		{0, 0}, {1, 1}, {3, 1}, {4, 1}, {5, 2}, {8, 2}, {9, 3}, {12, 3},
	}
	for _, c := range cases {
		if got := ceilQuarter(c.n); got != c.want {
			t.Errorf("ceilQuarter(%d) = %d, want %d", c.n, got, c.want)
		}
	}
}

func TestReverse(t *testing.T) {
	got := []string{"a", "b", "c", "d"}
	reverse(got)
	if want := []string{"d", "c", "b", "a"}; !reflect.DeepEqual(got, want) {
		t.Errorf("reverse = %#v, want %#v", got, want)
	}
	// Odd length and single element exercise the loop boundary.
	odd := []string{"x", "y", "z"}
	reverse(odd)
	if want := []string{"z", "y", "x"}; !reflect.DeepEqual(odd, want) {
		t.Errorf("reverse odd = %#v, want %#v", odd, want)
	}
}
