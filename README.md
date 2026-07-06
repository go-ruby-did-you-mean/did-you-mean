<p align="center"><img src="https://raw.githubusercontent.com/go-ruby-did-you-mean/brand/main/social/go-ruby-did-you-mean-did-you-mean.png" alt="go-ruby-did-you-mean/did-you-mean" width="720"></p>

# did-you-mean — go-ruby-did-you-mean

[![Docs](https://img.shields.io/badge/docs-mkdocs--material-DC2626)](https://go-ruby-did-you-mean.github.io/docs/)
[![License](https://img.shields.io/badge/license-BSD--3--Clause-blue)](LICENSE)
[![Go](https://img.shields.io/badge/go-1.26.4%2B-00ADD8)](https://go.dev/dl/)
[![Coverage](https://img.shields.io/badge/coverage-100%25-1a7f37)](#tests--coverage)

**A pure-Go (no cgo) reimplementation of the matcher at the heart of Ruby's
[`did_you_mean`](https://docs.ruby-lang.org/en/master/DidYouMean.html) standard
library** — MRI 4.0.5's `DidYouMean::SpellChecker#correct` and the two
string-distance metrics it ranks with, Jaro–Winkler and Levenshtein. Given a
mistyped word and a dictionary it returns Ruby's exact ranked list of spelling
suggestions. It is a faithful, byte-for-byte port of upstream `spell_checker.rb`,
`jaro_winkler.rb`, and `levenshtein.rb` — **without any Ruby runtime**.

It is the `did_you_mean` matcher for
[go-embedded-ruby](https://github.com/go-embedded-ruby/ruby), but is a
**standalone, reusable** module with no dependency on the Ruby runtime — a
sibling of [go-ruby-yaml](https://github.com/go-ruby-yaml/yaml) (Psych),
[go-ruby-regexp](https://github.com/go-ruby-regexp/regexp) (the Onigmo engine)
and [go-ruby-erb](https://github.com/go-ruby-erb/erb) (the ERB compiler).

> **What it is — and isn't.** Ranking suggestions for a misspelled name is pure,
> deterministic computation and needs **no interpreter**, so it lives here as
> pure Go. The interpreter-tied pieces of `did_you_mean` — the `NameError` /
> `NoMethodError` / `KeyError` hooks, the per-error checkers that gather the
> candidate names from a live object, and the message formatter — are the host's
> job; the Ruby runtime ([rbgo](https://github.com/go-embedded-ruby/ruby)) feeds
> this matcher a dictionary and renders its ranked result.

## Install

```sh
go get github.com/go-ruby-did-you-mean/did-you-mean
```

## Usage

```go
package main

import (
	"fmt"

	didyoumean "github.com/go-ruby-did-you-mean/did-you-mean"
)

func main() {
	methods := []string{"map", "select", "reject", "collect", "flatten"}

	fmt.Println(didyoumean.Correct("collekt", methods)) // [collect]
	fmt.Println(didyoumean.Correct("rerject", methods)) // [reject]
	fmt.Println(didyoumean.Correct("xyz", methods))     // []
}
```

## API

```go
// Correct returns the ranked spelling suggestions for input drawn from
// dictionary, exactly as Ruby's
// DidYouMean::SpellChecker.new(dictionary:).correct(input).
func Correct(input string, dictionary []string) []string
```

The `dictionary` order is significant: Ruby's stable sort preserves the input
order among equal-distance candidates, and the misspell fallback returns the
first such candidate — `Correct` mirrors that ordering.

## The algorithm

`Correct` is a line-for-line port of MRI's `SpellChecker#correct`:

1. **Normalize** input and every candidate — downcase and strip `@` sigils, so
   `@ivar` and `IVar` compare alike.
2. **Jaro–Winkler filter** — keep candidates whose Jaro–Winkler similarity to the
   input clears the threshold (`0.834` for inputs longer than 3 characters, else
   `0.77`), drop an exact match of the original input, and rank the survivors by
   that similarity, descending (a stable sort then reverse, matching Ruby's tie
   order).
3. **Levenshtein mistype filter** — keep the ranked candidates within an edit
   distance of `⌈len/4⌉` of the input.
4. **Misspell fallback** — if nothing survived, return the first ranked candidate
   whose Levenshtein distance is strictly less than the shorter of the two word
   lengths.

Both metrics — `Jaro`/`JaroWinkler.distance` and `Levenshtein.distance` — are
ported exactly, comparing Unicode code points (not bytes) like Ruby's
`String#each_codepoint`, so multibyte words (`café`, `naïve`) rank identically.

## Tests & coverage

The suite pairs deterministic, ruby-free golden vectors (captured from MRI 4.0.5,
which alone hold coverage at **100%**, so the qemu cross-arch and Windows lanes
pass the gate) with a **differential MRI oracle**: a broad corpus of typos,
transpositions, case variants, multibyte words, and no-match inputs is fed to the
system `ruby`'s `DidYouMean::SpellChecker#correct` and this package reproduces
every ranked array byte-for-byte. The oracle script `$stdout.binmode`s (and reads
its UTF-8 input off a binmode'd stdin) so Windows text-mode never pollutes the
multibyte bytes, gates itself on `RUBY_VERSION >= "4.0"`, and skips where `ruby`
is absent.

CGO-free, dependency-free, `gofmt` + `go vet` clean, and green across the three
host OSes (Linux, macOS, Windows) and the six 64-bit Go targets (amd64, arm64,
riscv64, loong64, ppc64le, s390x).

```sh
COVERPKG=$(go list ./... | paste -sd, -)
go test -race -coverpkg="$COVERPKG" -coverprofile=cover.out ./...
go tool cover -func=cover.out | tail -1   # 100.0%
```

## License

BSD-3-Clause — see [LICENSE](LICENSE). Copyright the go-ruby-did-you-mean/did-you-mean authors.

## WebAssembly

Being pure Go (CGO=0), this library also compiles to **WebAssembly** — both
`GOOS=js GOARCH=wasm` (browser / Node.js) and `GOOS=wasip1 GOARCH=wasm` (WASI).
CI builds both targets on every push, alongside the six 64-bit native/qemu arches.

```sh
GOOS=js     GOARCH=wasm go build ./...   # browser / Node
GOOS=wasip1 GOARCH=wasm go build ./...   # WASI (wasmtime, wasmer, wasmedge, …)
```
