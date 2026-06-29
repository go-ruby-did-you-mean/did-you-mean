// Copyright (c) 2026, the go-ruby-did-you-mean/did-you-mean authors
//
// SPDX-License-Identifier: BSD-3-Clause

package didyoumean

const (
	// jaroWinklerWeight is the prefix-scaling factor (MRI JaroWinkler::WEIGHT).
	jaroWinklerWeight = 0.1
	// jaroWinklerThreshold gates the prefix bonus (MRI JaroWinkler::THRESHOLD).
	jaroWinklerThreshold = 0.7
)

// jaroDistance returns the Jaro similarity of str1 and str2, a direct port of
// MRI's DidYouMean::Jaro.distance. MRI uses arbitrary-precision Integer bitmasks
// for the match flags; this uses []bool so words longer than 64 code points are
// handled exactly. Comparison is over Unicode code points.
func jaroDistance(str1, str2 string) float64 {
	r1 := []rune(str1)
	r2 := []rune(str2)
	// MRI swaps so str1 is the shorter (length1 <= length2).
	if len(r1) > len(r2) {
		r1, r2 = r2, r1
	}
	length1 := len(r1)
	length2 := len(r2)

	var m, t float64
	rng := 0
	if length2 > 3 {
		rng = length2/2 - 1
	}
	flags1 := make([]bool, length1)
	flags2 := make([]bool, length2)

	// Count matches within the search window.
	for i := 0; i < length1; i++ {
		last := i + rng
		j := 0
		if i >= rng {
			j = i - rng
		}
		for j <= last {
			// MRI indexes flags2[j] and str2_codepoints[j] unconditionally; j is
			// bounded by last = i+range which can exceed length2-1, where Ruby's
			// flags2[j] (bit test past the top) is 0 and str2_codepoints[j] is nil
			// — never a match. Guard the bounds to reproduce that.
			if j < length2 && !flags2[j] && r1[i] == r2[j] {
				flags2[j] = true
				flags1[i] = true
				m++
				break
			}
			j++
		}
	}

	// Count transpositions.
	k := 0
	for i := 0; i < length1; i++ {
		if flags1[i] {
			j := k
			index := k
			for j < length2 {
				index = j
				if flags2[j] {
					k = j + 1
					break
				}
				j++
			}
			if r1[i] != r2[index] {
				t++
			}
		}
	}
	t = float64(int(t) / 2)

	if m == 0 {
		return 0
	}
	return (m/float64(length1) + m/float64(length2) + (m-t)/m) / 3
}

// jaroWinklerDistance returns the Jaro–Winkler similarity, applying the common
// prefix bonus, a direct port of MRI's DidYouMean::JaroWinkler.distance.
func jaroWinklerDistance(str1, str2 string) float64 {
	jaro := jaroDistance(str1, str2)
	if jaro <= jaroWinklerThreshold {
		return jaro
	}

	r1 := []rune(str1)
	r2 := []rune(str2)
	prefixBonus := 0
	for _, c1 := range r1 {
		if prefixBonus < len(r2) && c1 == r2[prefixBonus] && prefixBonus < 4 {
			prefixBonus++
		} else {
			break
		}
	}
	return jaro + float64(prefixBonus)*jaroWinklerWeight*(1-jaro)
}
