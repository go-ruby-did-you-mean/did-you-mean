// Copyright (c) 2026, the go-ruby-did-you-mean/did-you-mean authors
//
// SPDX-License-Identifier: BSD-3-Clause

// Package didyoumean is a pure-Go (no cgo) reimplementation of the
// deterministic core of Ruby's did_you_mean stdlib: the spell-suggestion
// matcher (DidYouMean::SpellChecker#correct) and the two string-distance
// metrics it ranks with — Jaro–Winkler and Levenshtein.
//
// It is a faithful port of MRI 4.0.5's lib/did_you_mean/spell_checker.rb,
// jaro_winkler.rb, and levenshtein.rb, reproducing the exact ranked output
// Ruby's SpellChecker#correct returns — including the two Jaro–Winkler
// thresholds (0.77 / 0.834), the length-scaled Levenshtein mistype filter,
// the half-length misspell fallback, the "@"-stripping case-insensitive
// normalization, and the final ordering.
//
// The interpreter-tied pieces of did_you_mean (NameError/NoMethodError hooks,
// the per-error checkers, the formatter) belong to the Ruby host (rbgo); this
// module is the standalone, reusable, pure-compute matcher with no dependency
// on a Ruby runtime.
package didyoumean

import (
	"sort"
	"strings"
)

// Correct returns the ranked spelling suggestions for input drawn from
// dictionary, exactly as Ruby's DidYouMean::SpellChecker.new(dictionary:).
// correct(input) does.
//
// The dictionary order is significant: Ruby's stable sort preserves the input
// order among equal-distance candidates, and the half-length misspell fallback
// returns the first such candidate, so Correct mirrors that ordering.
func Correct(input string, dictionary []string) []string {
	normalizedInput := normalize(input)

	threshold := 0.77
	if len([]rune(normalizedInput)) > 3 {
		threshold = 0.834
	}

	// Keep candidates whose Jaro–Winkler distance to the input clears the
	// threshold, then drop an exact match of the original (un-normalized) input.
	var words []string
	for _, word := range dictionary {
		if jaroWinklerDistance(normalize(word), normalizedInput) >= threshold {
			words = append(words, word)
		}
	}
	filtered := words[:0:0]
	for _, word := range words {
		if input != word {
			filtered = append(filtered, word)
		}
	}
	words = filtered

	// Rank by Jaro–Winkler distance to the input, descending. MRI sorts
	// ascending then reverses; replicate that exact tie order with a stable sort
	// keyed on the negated distance is NOT equivalent because reverse flips the
	// stable order of ties — so sort-then-reverse explicitly.
	sort.SliceStable(words, func(i, j int) bool {
		return jaroWinklerDistance(words[i], normalizedInput) <
			jaroWinklerDistance(words[j], normalizedInput)
	})
	reverse(words)

	// Correct mistypes: a Levenshtein distance within a quarter of the input
	// length (rounded up).
	mistypeThreshold := ceilQuarter(len([]rune(normalizedInput)))
	var corrections []string
	for _, c := range words {
		if levenshteinDistance(normalize(c), normalizedInput) <= mistypeThreshold {
			corrections = append(corrections, c)
		}
	}

	// Correct misspells: if nothing matched, fall back to the single best
	// candidate within half the (shorter) word length.
	if len(corrections) == 0 {
		inputLen := len([]rune(normalizedInput))
		for _, word := range words {
			nw := normalize(word)
			length := inputLen
			if wl := len([]rune(nw)); wl < length {
				length = wl
			}
			if levenshteinDistance(nw, normalizedInput) < length {
				corrections = []string{word}
				break
			}
		}
	}

	return corrections
}

// normalize lower-cases the word and strips "@" sigils, mirroring MRI's private
// SpellChecker#normalize (str.downcase.tr("@", "")).
func normalize(s string) string {
	return strings.ReplaceAll(strings.ToLower(s), "@", "")
}

// reverse reverses a slice in place (MRI's Array#reverse! on the ranked list).
func reverse(s []string) {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
}

// ceilQuarter returns (n * 0.25).ceil for a non-negative integer n, matching
// MRI's `(normalized_input.length * 0.25).ceil`.
func ceilQuarter(n int) int {
	return (n + 3) / 4
}
