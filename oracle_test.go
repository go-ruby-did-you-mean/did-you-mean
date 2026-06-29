// Copyright (c) 2026, the go-ruby-did-you-mean/did-you-mean authors
//
// SPDX-License-Identifier: BSD-3-Clause

package didyoumean

import (
	"encoding/json"
	"os/exec"
	"reflect"
	"strings"
	"testing"
)

// rubyBin locates a usable `ruby` whose did_you_mean matches the MRI 4.0+ core
// this package targets. The oracle tests skip themselves when ruby is absent
// (the qemu cross-arch lanes and the Windows lane) or when it is older than 4.0,
// so the deterministic suite alone drives the 100% gate there.
func rubyBin(t *testing.T) string {
	t.Helper()
	path, err := exec.LookPath("ruby")
	if err != nil {
		t.Skip("ruby not on PATH; skipping MRI oracle")
	}
	out, err := exec.Command(path, "-e", "print RUBY_VERSION").Output()
	if err != nil {
		t.Skipf("cannot query ruby version: %v", err)
	}
	if v := string(out); v < "4.0" {
		t.Skipf("ruby %s < 4.0; did_you_mean core differs, skipping MRI oracle", v)
	}
	return path
}

// rubyCorrect runs MRI's DidYouMean::SpellChecker#correct for a batch of
// (dictionary, input) pairs and returns the resulting arrays in order. The
// script $stdout.binmode's (and reads its UTF-8 JSON argument off stdin, also
// binmode'd) so Windows text-mode never pollutes the multibyte bytes — the
// go-ruby-erb lesson — and is the single source of truth the oracle diffs.
func rubyCorrect(t *testing.T, bin string, dicts [][]string, inputs []string) [][]string {
	t.Helper()
	type req struct {
		Dicts  [][]string `json:"dicts"`
		Inputs []string   `json:"inputs"`
	}
	payload, err := json.Marshal(req{dicts, inputs})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	const script = `
$stdout.binmode
$stdin.binmode
require "did_you_mean"
require "json"
data = JSON.parse($stdin.read.force_encoding("UTF-8"))
out = data["dicts"].zip(data["inputs"]).map do |dict, input|
  DidYouMean::SpellChecker.new(dictionary: dict).correct(input)
end
print JSON.generate(out)
`
	cmd := exec.Command(bin, "-e", script)
	cmd.Stdin = strings.NewReader(string(payload))
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("ruby oracle error: %v\noutput:\n%s", err, out)
	}
	var got [][]string
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("decode oracle output: %v\nraw:\n%s", err, out)
	}
	return got
}

// TestOracleMatchesMRI feeds the golden corpus (and a broad set of extra typos,
// transpositions, case variants, multibyte words, and empty-result cases) to the
// live MRI did_you_mean and asserts this package reproduces every ranked array
// byte-for-byte.
func TestOracleMatchesMRI(t *testing.T) {
	bin := rubyBin(t)

	short := []string{"first", "last", "least", "lost"}
	caps := []string{"Float", "Integer", "String", "Symbol", "Array", "Hash"}
	uni := []string{"café", "naïve", "résumé", "cafe", "naive", "resume"}
	long := []string{"configuration", "configurations", "config", "configure", "configured"}

	type probe struct {
		dict  []string
		input string
	}
	var probes []probe
	add := func(d []string, ins ...string) {
		for _, in := range ins {
			probes = append(probes, probe{d, in})
		}
	}
	// The deterministic golden cases, re-checked live.
	for _, c := range correctCases {
		probes = append(probes, probe{c.dict, c.input})
	}
	// Method-name typos, transpositions, and a no-match input.
	add(rubyMethods, "collekt", "collec", "rerject", "recjet", "slect", "selct",
		"flaten", "compackt", "injekt", "detekt", "xyz", "zzzzz", "")
	// Short dictionary, threshold 0.77.
	add(short, "fst", "lst", "leist", "lest", "lis", "lost", "frist")
	// Case-insensitivity over capitalized names.
	add(caps, "flot", "Flot", "integr", "Integr", "strng", "Symbl", "aray", "haash")
	// Multibyte code points.
	add(uni, "cafe", "café", "cafw", "naive", "naïv", "resme", "résumé")
	// Long words, threshold 0.834.
	add(long, "configurtion", "configurations", "confgi", "configred", "configuratio")

	dicts := make([][]string, len(probes))
	inputs := make([]string, len(probes))
	for i, p := range probes {
		dicts[i] = p.dict
		inputs[i] = p.input
	}

	want := rubyCorrect(t, bin, dicts, inputs)
	if len(want) != len(probes) {
		t.Fatalf("oracle returned %d results, want %d", len(want), len(probes))
	}

	for i, p := range probes {
		got := Correct(p.input, p.dict)
		w := want[i]
		if len(got) == 0 && len(w) == 0 {
			continue
		}
		if !reflect.DeepEqual(got, w) {
			t.Errorf("Correct(%q, %v):\n  go  =%#v\n  ruby=%#v", p.input, p.dict, got, w)
		}
	}
}
