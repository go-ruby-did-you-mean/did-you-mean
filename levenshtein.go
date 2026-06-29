// Copyright (c) 2026, the go-ruby-did-you-mean/did-you-mean authors
//
// SPDX-License-Identifier: BSD-3-Clause

package didyoumean

// levenshteinDistance returns the edit distance between str1 and str2, a direct
// port of MRI's DidYouMean::Levenshtein.distance (itself from the Text gem). It
// compares Unicode code points, like Ruby's String#each_codepoint.
func levenshteinDistance(str1, str2 string) int {
	r1 := []rune(str1)
	r2 := []rune(str2)
	n := len(r1)
	m := len(r2)
	if n == 0 {
		return m
	}
	if m == 0 {
		return n
	}

	// d holds the previous matrix row, seeded 0..m.
	d := make([]int, m+1)
	for k := range d {
		d[k] = k
	}

	x := 0
	for idx, char1 := range r1 {
		i := idx + 1 // Ruby's with_index(1)
		for j := 0; j < m; j++ {
			cost := 1
			if char1 == r2[j] {
				cost = 0
			}
			x = min3(
				d[j+1]+1,  // insertion
				i+1,       // deletion
				d[j]+cost, // substitution
			)
			d[j] = i
			i = x
		}
		d[m] = x
	}

	return x
}

// min3 returns the minimum of three integers, matching MRI's hand-rolled min3.
func min3(a, b, c int) int {
	if a < b && a < c {
		return a
	}
	if b < c {
		return b
	}
	return c
}
