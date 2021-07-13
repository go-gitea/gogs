# go-enry [![GoDoc](https://godoc.org/github.com/go-enry/go-enry?status.svg)](https://pkg.go.dev/github.com/go-enry/go-enry/v2) [![Test](https://github.com/go-enry/go-enry/workflows/Test/badge.svg)](https://github.com/go-enry/go-enry/actions?query=workflow%3ATest+branch%3Amaster) [![codecov](https://codecov.io/gh/go-enry/go-enry/branch/master/graph/badge.svg)](https://codecov.io/gh/go-enry/go-enry)

Programming language detector and toolbox to ignore binary or vendored files. _enry_, started as a port to _Go_ of the original [Linguist](https://github.com/github/linguist) _Ruby_ library, that has an improved _2x performance_.

- [CLI](#cli)
- [Library](#library)
  - [Use cases](#use-cases)
    - [By filename](#by-filename)
    - [By text](#by-text)
    - [By file](#by-file)
    - [Filtering](#filtering-vendoring-binaries-etc)
    - [Coloring](#language-colors-and-groups)
  - [Languages](#languages)
    - [Go](#go)
    - [Java bindings](#java-bindings)
    - [Python bindings](#python-bindings)
    - [Rust bindings](#rust-bindings)
- [Divergences from linguist](#divergences-from-linguist)
- [Benchmarks](#benchmarks)
- [Why Enry?](#why-enry)
- [Development](#development)
  - [Sync with github/linguist upstream](#sync-with-githublinguist-upstream)
- [Misc](#misc)
- [License](#license)

# CLI

The CLI binary is hosted in a separate repository [go-enry/enry](https://github.com/go-enry/enry).

# Library

_enry_ is also a Go library for guessing a programming language that exposes API through FFI to multiple programming environments.

## Use cases

_enry_ guesses a programming language using a sequence of matching _strategies_ that are
applied progressively to narrow down the possible options. Each _strategy_ varies on the type
of input data that it needs to make a decision: file name, extension, the first line of the file, the full content of the file, etc.

Depending on available input data, enry API can be roughly divided into the next categories or use cases.

### By filename

Next functions require only a name of the file to make a guess:

- `GetLanguageByExtension` uses only file extension (wich may be ambiguous)
- `GetLanguageByFilename` useful for cases like `.gitignore`, `.bashrc`, etc
- all [filtering helpers](#filtering)

Please note that such guesses are expected not to be very accurate.

### By text

To make a guess only based on the content of the file or a text snippet, use

- `GetLanguageByShebang` reads only the first line of text to identify the [shebang](<https://en.wikipedia.org/wiki/Shebang_(Unix)>).
- `GetLanguageByModeline` for cases when Vim/Emacs modeline e.g. `/* vim: set ft=cpp: */` may be present at a head or a tail of the text.
- `GetLanguageByClassifier` uses a Bayesian classifier trained on all the `./samples/` from Linguist.

  It usually is a last-resort strategy that is used to disambiguate the guess of the previous strategies, and thus it requires a list of "candidate" guesses. One can provide a list of all known languages - keys from the `data.LanguagesLogProbabilities` as possible candidates if more intelligent hypotheses are not available, at the price of possibly suboptimal accuracy.

### By file

The most accurate guess would be one when both, the file name and the content are available:

- `GetLanguagesByContent` only uses file extension and a set of regexp-based content heuristics.
- `GetLanguages` uses the full set of matching strategies and is expected to be most accurate.

### Filtering: vendoring, binaries, etc

_enry_ expose a set of file-level helpers `Is*` to simplify filtering out the files that are less interesting for the purpose of source code analysis:

- `IsBinary`
- `IsVendor`
- `IsConfiguration`
- `IsDocumentation`
- `IsDotFile`
- `IsImage`
- `IsTest`
- `IsGenerated`

### Language colors and groups

_enry_ exposes function to get language color to use for example in presenting statistics in graphs:

- `GetColor`
- `GetLanguageGroup` can be used to group similar languages together e.g. for `Less` this function will return `CSS`

## Languages

### Go

In a [Go module](https://github.com/golang/go/wiki/Modules),
import `enry` to the module by running:

```sh
go get github.com/go-enry/go-enry/v2
```

The rest of the examples will assume you have either done this or fetched the
library into your `GOPATH`.

```go
// The examples here and below assume you have imported the library.
import "github.com/go-enry/go-enry/v2"

lang, safe := enry.GetLanguageByExtension("foo.go")
fmt.Println(lang, safe)
// result: Go true

lang, safe := enry.GetLanguageByContent("foo.m", []byte("<matlab-code>"))
fmt.Println(lang, safe)
// result: Matlab true

lang, safe := enry.GetLanguageByContent("bar.m", []byte("<objective-c-code>"))
fmt.Println(lang, safe)
// result: Objective-C true

// all strategies together
lang := enry.GetLanguage("foo.cpp", []byte("<cpp-code>"))
// result: C++ true
```

Note that the returned boolean value `safe` is `true` if there is only one possible language detected.

A plural version of the same API allows getting a list of all possible languages for a given file.

```go
langs := enry.GetLanguages("foo.h",  []byte("<cpp-code>"))
// result: []string{"C", "C++", "Objective-C}

langs := enry.GetLanguagesByExtension("foo.asc", []byte("<content>"), nil)
// result: []string{"AGS Script", "AsciiDoc", "Public Key"}

langs := enry.GetLanguagesByFilename("Gemfile", []byte("<content>"), []string{})
// result: []string{"Ruby"}
```

### Java bindings

Generated Java bindings using a C shared library and JNI are available under [`java`](https://github.com/go-enry/go-enry/blob/master/java).

A library is published on Maven as [tech.sourced:enry-java](https://mvnrepository.com/artifact/tech.sourced/enry-java) for macOS and linux platforms. Windows support is planned under [src-d/enry#150](https://github.com/src-d/enry/issues/150).

### Python bindings

Generated Python bindings using a C shared library and cffi are WIP under [src-d/enry#154](https://github.com/src-d/enry/issues/154).

A library is going to be published on pypi as [enry](https://pypi.org/project/enry/) for
macOS and linux platforms. Windows support is planned under [src-d/enry#150](https://github.com/src-d/enry/issues/150).

### Rust bindings

Generated Rust bindings using a C static library are available at https://github.com/go-enry/rs-enry.


## Divergences from Linguist

The `enry` library is based on the data from `github/linguist` version **v7.14.0**.

Parsing [linguist/samples](https://github.com/github/linguist/tree/master/samples) the following `enry` results are different from the Linguist:

- [Heuristics for ".txt" extension](https://github.com/github/linguist/blob/8083cb5a89cee2d99f5a988f165994d0243f0d1e/lib/linguist/heuristics.yml#L521) in Vim Help File could not be parsed, due to unsupported negative lookahead in RE2 regexp engine.

- [Heuristics for ".sol" extension](https://github.com/github/linguist/blob/8083cb5a89cee2d99f5a988f165994d0243f0d1e/lib/linguist/heuristics.yml#L464) in Solidity could not be parsed, due to unsupported negative lookahead in RE2 regexp engine.

- [Heuristics for ".es" extension](https://github.com/github/linguist/blob/e761f9b013e5b61161481fcb898b59721ee40e3d/lib/linguist/heuristics.yml#L103) in JavaScript could not be parsed, due to unsupported backreference in RE2 regexp engine.

- [Heuristics for ".rno" extension](https://github.com/github/linguist/blob/3a1bd3c3d3e741a8aaec4704f782e06f5cd2a00d/lib/linguist/heuristics.yml#L365) in RUNOFF could not be parsed, due to unsupported lookahead in RE2 regexp engine.

- [Heuristics for ".inc" extension](https://github.com/github/linguist/blob/f0e2d0d7f1ce600b2a5acccaef6b149c87d8b99c/lib/linguist/heuristics.yml#L222) in NASL could not be parsed, due to unsupported possessive quantifier in RE2 regexp engine.

- [Heuristics for ".as" extension](https://github.com/github/linguist/blob/223c00bb80eff04788e29010f98c5778993d2b2a/lib/linguist/heuristics.yml#L67) in ActionScript could not be parsed, due to unsupported positive lookahead in RE2 regexp engine.

- As of [Linguist v5.3.2](https://github.com/github/linguist/releases/tag/v5.3.2) it is using [flex-based scanner in C for tokenization](https://github.com/github/linguist/pull/3846). Enry still uses [extract_token](https://github.com/github/linguist/pull/3846/files#diff-d5179df0b71620e3fac4535cd1368d15L60) regex-based algorithm. See [#193](https://github.com/src-d/enry/issues/193).

- Bayesian classifier can't distinguish "SQL" from "PLpgSQL. See [#194](https://github.com/src-d/enry/issues/194).

- Overriding languages and types though `.gitattributes` is not yet supported. See [#18](https://github.com/src-d/enry/issues/18).

- `enry` CLI output does NOT exclude `.gitignore`ed files and git submodules, as Linguist does

In all the cases above that have an issue number - we plan to update enry to match Linguist behavior.

## Benchmarks

Enry's language detection has been compared with Linguist's on [_linguist/samples_](https://github.com/github/linguist/tree/master/samples).

We got these results:

![histogram](benchmarks/histogram/distribution.png)

The histogram shows the _number of files_ (y-axis) per _time interval bucket_ (x-axis).
Most of the files were detected faster by enry.

There are several cases where enry is slower than Linguist due to
Go regexp engine being slower than Ruby's on, wich is based on [oniguruma](https://github.com/kkos/oniguruma) library, written in C.

See [instructions](#misc) for running enry with oniguruma.

## Why Enry?

In the movie [My Fair Lady](https://en.wikipedia.org/wiki/My_Fair_Lady), [Professor Henry Higgins](http://www.imdb.com/character/ch0011719/) is a linguist who at the very beginning of the movie enjoys guessing the origin of people based on their accent.

"Enry Iggins" is how [Eliza Doolittle](http://www.imdb.com/character/ch0011720/), [pronounces](https://www.youtube.com/watch?v=pwNKyTktDIE) the name of the Professor.

## Development

To run the tests use:

    go test ./...

Setting `ENRY_TEST_REPO` to the path to existing checkout of Linguist will avoid cloning it and sepeed tests up.
Setting `ENRY_DEBUG=1` will provide insight in the Bayesian classifier building done by `make code-generate`.

### Sync with github/linguist upstream

_enry_ re-uses parts of the original [github/linguist](https://github.com/github/linguist) to generate internal data structures.
In order to update to the latest release of linguist do:

```bash
$ git clone https://github.com/github/linguist.git .linguist
$ cd .linguist; git checkout <release-tag>; cd ..

# put the new release's commit sha in the generator_test.go (to re-generate .gold test fixtures)
# https://github.com/go-enry/go-enry/blob/13d3d66d37a87f23a013246a1b0678c9ee3d524b/internal/code-generator/generator/generator_test.go#L18

$ make code-generate
```

To stay in sync, enry needs to be updated when a new release of the linguist includes changes to any of the following files:

- [languages.yml](https://github.com/github/linguist/blob/master/lib/linguist/languages.yml)
- [heuristics.yml](https://github.com/github/linguist/blob/master/lib/linguist/heuristics.yml)
- [vendor.yml](https://github.com/github/linguist/blob/master/lib/linguist/vendor.yml)
- [documentation.yml](https://github.com/github/linguist/blob/master/lib/linguist/documentation.yml)

There is no automation for detecting the changes in the linguist project, so this process above has to be done manually from time to time.

When submitting a pull request syncing up to a new release, please make sure it only contains the changes in
the generated files (in [data](https://github.com/go-enry/go-enry/blob/master/data) subdirectory).

Separating all the necessary "manual" code changes to a different PR that includes some background description and an update to the documentation on ["divergences from linguist"](#divergences-from-linguist) is very much appreciated as it simplifies the maintenance (review/release notes/etc).

## Misc

<details>
  <summary>Running a benchmark & faster regexp engine</summary>

### Benchmark

All benchmark scripts are in [_benchmarks_](https://github.com/go-enry/go-enry/blob/master/benchmarks) directory.

#### Dependencies

As benchmarks depend on Ruby and Github-Linguist gem make sure you have:

- Ruby (e.g using [`rbenv`](https://github.com/rbenv/rbenv)), [`bundler`](https://bundler.io/) installed
- Docker
- [native dependencies](https://github.com/github/linguist/#dependencies) installed
- Build the gem `cd .linguist && bundle install && rake build_gem && cd -`
- Install it `gem install --no-rdoc --no-ri --local .linguist/github-linguist-*.gem`

#### Quick benchmark

To run quicker benchmarks

    make benchmarks

to get average times for the primary detection function and strategies for the whole samples set. If you want to see measures per sample file use:

    make benchmarks-samples

#### Full benchmark

If you want to reproduce the same benchmarks as reported above:

- Make sure all [dependencies](#benchmark-dependencies) are installed
- Install [gnuplot](http://gnuplot.info) (in order to plot the histogram)
- Run `ENRY_TEST_REPO="$PWD/.linguist" benchmarks/run.sh` (takes ~15h)

It will run the benchmarks for enry and Linguist, parse the output, create csv files and plot the histogram.

### Faster regexp engine (optional)

[Oniguruma](https://github.com/kkos/oniguruma) is CRuby's regular expression engine.
It is very fast and performs better than the one built into Go runtime. _enry_ supports swapping
between those two engines thanks to [rubex](https://github.com/moovweb/rubex) project.
The typical overall speedup from using Oniguruma is 1.5-2x. However, it requires CGo and the external shared library.
On macOS with [Homebrew](https://brew.sh/), it is:

```
brew install oniguruma
```

On Ubuntu, it is

```
sudo apt install libonig-dev
```

To build enry with Oniguruma regexps use the `oniguruma` build tag

```
go get -v -t --tags oniguruma ./...
```

and then rebuild the project.

</details>

## License

Apache License, Version 2.0. See [LICENSE](LICENSE)
