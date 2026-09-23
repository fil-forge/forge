package stack

import (
	"io"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/fil-forge/forge/smelt/pkg/generate"
	"github.com/fil-forge/forge/smelt/pkg/manifest"
	"gopkg.in/yaml.v3"
)

// The published image set is stated in three places, and they must agree:
//
//   - `.env.published`, which supplies them to `make up`;
//   - publishedImages, which supplies them to WithPublishedImages();
//   - envImageOptions, which lets a Go caller override them by the same names.
//
// They had already drifted: indexing-service was added to publishedImages and
// not to .env.published, so `make up` failed at `compose up` with INDEXER_IMAGE
// unset, for as long as the indexer had been in the stack. Three comments in
// this repository asserted that could not happen silently, on the grounds that
// a missing entry fails at `compose up`. It does -- and nothing in CI runs
// `make up`, so "not silent" and "not noticed" were the same thing.
//
// WHAT THIS DELIBERATELY DOES NOT CHECK, and how to get it back. It does not
// read the compose files, so it does not assert that compose requires exactly
// this set and no more. An earlier revision did, and the cost was not worth it:
// six review rounds, every one finding the same defect -- a deriver reading
// something other than what its consumer reads. It compared the lists as sets
// rather than pairs; matched only `${X:?}` and not `${X?}`; missed `include:`
// targets and `compose.yaml`; grepped raw bytes, so `#` comments and `$$`
// escapes counted; visited YAML map keys, which compose never interpolates;
// read non-compose YAML and the generator's own tests; and omitted the four
// `*.override.*` names compose also auto-discovers. Each fix was right and the
// rate was the signal: 285 of its 433 lines reimplemented a slice of compose's
// grammar, and compose's grammar is bigger than what was written down.
//
// It was also not load-bearing. Measured by stubbing that derivation out to a
// constant: the original bug (INDEXER_IMAGE absent from .env.published) and the
// swapped-values case both still fail, because the expected set comes from the
// two Go tables and never from the compose files.
//
// If the stricter property is wanted, ask compose rather than reimplementing
// it: run `docker compose config` with an empty environment and read back the
// variable it names as missing, repeating until it stops. That is exact by
// construction, needs no daemon, and was measured at ~1.4s over 9 invocations
// at the smelt root. It costs a dependency on the compose binary in
// `unit smelt`, which is the trade to weigh. Do NOT use `docker compose config
// --variables`: it under-reports, missing a required variable in a
// non-extended service of an `extends.file` target that compose itself
// enforces.

const imageSentinel = "SENTINEL-published-images-test"

// fieldReachedBy applies mutate to a fresh config and returns the name of the
// single field it set. Pairing by BEHAVIOUR rather than by a name convention:
// nothing has to agree about how PIRI_IMAGE relates to piriImage, and a
// renamed field cannot silently break the join.
func fieldReachedBy(t *testing.T, mutate func(*config)) string {
	t.Helper()
	var c config
	mutate(&c)

	v := reflect.ValueOf(c)
	var found []string
	for i := 0; i < v.NumField(); i++ {
		if v.Field(i).Kind() == reflect.String && v.Field(i).String() == imageSentinel {
			found = append(found, v.Type().Field(i).Name)
		}
	}
	require.Len(t, found, 1, "expected exactly one config field to be set")
	return found[0]
}

func readEnvPublished(t *testing.T, path string) map[string]string {
	t.Helper()
	b, err := os.ReadFile(path)
	require.NoError(t, err)

	out := map[string]string{}
	for i, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		require.True(t, ok, "%s:%d: not KEY=VALUE: %q", path, i+1, line)
		// A repeat binding is a real defect and invisible to any set
		// comparison: an earlier shell version of this check passed on a file
		// with PIRI_IMAGE bound twice to different images.
		_, dup := out[k]
		require.False(t, dup, "%s:%d: %s is set more than once", path, i+1, k)
		out[k] = v
	}
	return out
}

func TestPublishedImageSetAgrees(t *testing.T) {
	// Pair each override variable and each published reference to the config
	// field it reaches, then join on that field.
	varToField := map[string]string{}
	for _, e := range envImageOptions {
		varToField[e.env] = fieldReachedBy(t, func(c *config) { e.opt(imageSentinel)(c) })
	}
	fieldToRef := map[string]string{}
	for _, p := range publishedImages {
		f := fieldReachedBy(t, func(c *config) { *p.get(c) = imageSentinel })
		_, dup := fieldToRef[f]
		require.False(t, dup, "publishedImages sets %s twice", f)
		fieldToRef[f] = p.ref
	}

	// The images this repository builds are those with both an override option
	// and a published reference.
	var wantVars []string
	for v, f := range varToField {
		if _, built := fieldToRef[f]; built {
			wantVars = append(wantVars, v)
		}
	}
	sort.Strings(wantVars)
	require.NotEmpty(t, wantVars, "no image has both an option and a published reference")

	env := readEnvPublished(t, filepath.Join("..", "..", ".env.published"))
	var gotEnv []string
	for v := range env {
		gotEnv = append(gotEnv, v)
	}
	sort.Strings(gotEnv)

	require.Equal(t, wantVars, gotEnv,
		".env.published does not supply exactly the images this repository builds")

	// Every published reference must be reachable by an override variable.
	// Without this a publishedImages entry whose config field no
	// envImageOptions entry reaches drops out of wantVars silently -- not
	// required in .env.published, reference never compared. That is the
	// symmetric twin of the drift this test exists for, and it was the one
	// way two of the three could genuinely disagree and pass.
	reached := map[string]bool{}
	for _, f := range varToField {
		reached[f] = true
	}
	for f := range fieldToRef {
		require.True(t, reached[f],
			"publishedImages publishes %s, which no envImageOptions entry reaches", f)
	}

	// Comparing the REFERENCES pairwise, not the two sets. A set comparison
	// passes on a file where PIRI_IMAGE and HILT_IMAGE have been swapped --
	// each service booting the other's image -- which is how the shell version
	// of this check reported agreement on a broken tree.
	for _, v := range wantVars {
		require.Equal(t, fieldToRef[varToField[v]], env[v],
			"%s in .env.published names a different image than publishedImages does", v)
	}
}

// TestBuiltImagesAreRequiredNotDefaulted checks that each image this
// repository builds is a REQUIRED compose interpolation, not one with a
// fallback -- and that no defaulted form of one exists in the project.
//
// WHY THIS CAN READ THE COMPOSE FILES WHEN THE CHECK ABOVE COULD NOT. It asks
// a different question, and the direction of the question decides which way
// incompleteness errs. "Is each of these known names required somewhere?"
// FAILS when a file is missed -- a false failure. "What is the complete set of
// required names?" PASSES when a file is missed, and six review rounds went
// into chasing every way that could happen.
//
// So under-reading is safe here and OVER-reading is not, which is what the
// file set and the two exclusions below are for. An earlier revision got the
// direction argument right and then over-read three ways:
//
//   - it counted `$${X:?}`, which is compose's escape for a literal `$` and
//     requires nothing. Blanked before matching now, as the deleted derivation
//     already did -- the unfixed half of that fix.
//   - it counted any compose-SHAPED filename, including
//     systems/telemetry/compose.yml and systems/stress-tester/compose.yml,
//     which no project loads: they are in no include list, no Makefile target
//     and no embed set. A required occurrence planted in one satisfied the
//     check for a name defaulted everywhere compose looks.
//   - it scanned the generator's SOURCE, so a string literal it never emits
//     counted. That is round six's finding, reintroduced.
//
// The file set is now the project `make up` runs: the root compose file and
// its `include:` targets, transitively. Missing one is safe by the direction
// above, so this needs none of the auto-discovery or `extends:` machinery the
// deleted derivation needed. And the generator is CALLED rather than read, so
// what counts is what it emits.
//
// WHAT IT PROTECTS. `.env.published`'s header and smelt/CLAUDE.md both say the
// required form exists because a silent default meant a caller who forgot an
// override booted a published image and never heard about it. Downgrading
// `${HILT_IMAGE:?...}` to `${HILT_IMAGE:-ghcr.io/fil-forge/hilt:main}` puts
// that back, and nothing else notices: check-stack-images.sh skips every
// `${...}` value by design, and no Go path compares the two forms.
func TestBuiltImagesAreRequiredNotDefaulted(t *testing.T) {
	root := filepath.Join("..", "..")

	built := map[string]bool{}
	for _, e := range envImageOptions {
		f := fieldReachedBy(t, func(c *config) { e.opt(imageSentinel)(c) })
		for _, p := range publishedImages {
			if fieldReachedBy(t, func(c *config) { *p.get(c) = imageSentinel }) == f {
				built[e.env] = true
			}
		}
	}
	require.NotEmpty(t, built)

	// `:?` errors when unset or empty, `?` when unset. Either is required.
	// `:-` and `-` supply a default, which is what must not appear.
	required := regexp.MustCompile(`\$\{([A-Z0-9_]+_IMAGE):?\?`)
	defaulted := regexp.MustCompile(`\$\{([A-Z0-9_]+_IMAGE):?-`)

	isRequired := map[string]bool{}
	isDefaulted := map[string]string{} // variable -> where

	scan := func(where, text string) {
		// `$$` is compose's escape for a literal `$`. Blank it before matching
		// so `$${X:?}` cannot count as a requirement.
		text = strings.ReplaceAll(text, "$$", "\x00\x00")
		for _, m := range required.FindAllStringSubmatch(text, -1) {
			isRequired[m[1]] = true
		}
		for _, m := range defaulted.FindAllStringSubmatch(text, -1) {
			if built[m[1]] {
				isDefaulted[m[1]] = where
			}
		}
	}

	// The project `make up` runs: the root compose file and its include
	// targets, transitively.
	seen := map[string]bool{}
	queue := []string{filepath.Join(root, "compose.yml")}
	for len(queue) > 0 {
		path := queue[0]
		queue = queue[1:]
		if seen[path] {
			continue
		}
		seen[path] = true

		b, err := os.ReadFile(path)
		if err != nil {
			// The one include that is legitimately absent is generated/, which
			// is gitignored output; its content is covered by calling the
			// generator below.
			require.True(t, os.IsNotExist(err) && strings.Contains(path, "generated"),
				"reading %s: %v", path, err)
			continue
		}
		dec := yaml.NewDecoder(strings.NewReader(string(b)))
		for {
			var doc any
			if derr := dec.Decode(&doc); derr != nil {
				require.ErrorIs(t, derr, io.EOF, "parsing %s", path)
				break
			}
			// Parsed, not scanned: a commented-out `${X:?}` left above a
			// downgraded line must not count, and does not.
			walkStrings(doc, func(v string) { scan(path, v) })
			queue = append(queue, includeTargets(filepath.Dir(path), doc)...)
		}
	}

	// piri's service definition is emitted, not written down. CALLED, not
	// read: a `${PIRI_IMAGE:?}` string literal the generator never emits must
	// not count.
	emitted, err := generate.GeneratePiriCompose([]manifest.ResolvedPiriNode{
		{Name: "piri-0", Index: 0},
	})
	require.NoError(t, err)
	scan("pkg/generate (emitted)", string(emitted))

	for v := range built {
		require.True(t, isRequired[v],
			"%s is built here but is not a required compose interpolation anywhere "+
				"in the project; a `${%s:-default}` form silently boots a published "+
				"image when the caller forgets an override", v, v)
		require.Empty(t, isDefaulted[v],
			"%s is built here but has a DEFAULTED form in %s; that is the silent "+
				"fallback the required form exists to prevent", v, isDefaulted[v])
	}
}

// includeTargets returns the `include:` paths a compose document names,
// resolved against its own directory. Entries are a path, a list of paths, or
// a mapping with `path:` holding either.
func includeTargets(dir string, doc any) []string {
	m, ok := doc.(map[string]any)
	if !ok {
		return nil
	}
	list, ok := m["include"].([]any)
	if !ok {
		return nil
	}
	var out []string
	add := func(v any) {
		if p, ok := v.(string); ok {
			out = append(out, filepath.Join(dir, p))
		}
	}
	for _, e := range list {
		switch v := e.(type) {
		case string:
			add(v)
		case map[string]any:
			switch p := v["path"].(type) {
			case string:
				add(p)
			case []any:
				for _, q := range p {
					add(q)
				}
			}
		}
	}
	return out
}

// walkStrings visits every string scalar VALUE. Keys are not visited: compose
// does not interpolate them -- measured, `docker compose config` exits 0 with
// the variable unset and re-escapes the `$` to `$$`.
func walkStrings(n any, fn func(string)) {
	switch v := n.(type) {
	case string:
		fn(v)
	case []any:
		for _, e := range v {
			walkStrings(e, fn)
		}
	case map[string]any:
		for _, e := range v {
			walkStrings(e, fn)
		}
	case map[any]any:
		for _, e := range v {
			walkStrings(e, fn)
		}
	}
}
