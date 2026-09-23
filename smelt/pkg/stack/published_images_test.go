package stack

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
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
// fallback.
//
// WHY THIS IS SAFE TO DO BY READING THE COMPOSE FILES WHEN THE CHECK ABOVE IS
// NOT. It asks a different question, and the direction of the question decides
// which way incompleteness errs. "Does each of these known names appear
// required somewhere?" fails when a file is missed -- a false failure. "What
// is the complete set of required names?" PASSES when a file is missed, and
// six review rounds went into chasing every way that could happen. Under-
// reading is safe here; over-reading is not, which is why this looks only at
// files named like compose files and not at every YAML under the root.
//
// Comments cannot mask a downgrade, because the YAML is parsed rather than
// scanned: a commented-out `${X:?}` left above a defaulted line is not in the
// parsed document at all.
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
	required := regexp.MustCompile(`\$\{([A-Z0-9_]+_IMAGE):?\?`)
	found := map[string]bool{}

	isComposeFile := func(name string) bool {
		return strings.HasPrefix(name, "compose.") || strings.HasPrefix(name, "docker-compose.")
	}

	require.NoError(t, filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		switch {
		case d.IsDir():
			return nil
		case isComposeFile(d.Name()) &&
			(strings.HasSuffix(path, ".yml") || strings.HasSuffix(path, ".yaml")):
		case strings.HasPrefix(path, filepath.Join(root, "pkg", "generate")) &&
			strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go"):
			// piri's service definition is emitted, not written down. PARSED,
			// not scanned: a raw read matches inside a `//` comment, and
			// over-reading is the unsafe direction for this question -- a
			// commented-out `${PIRI_IMAGE:?}` would let a downgraded live one
			// pass.
			f, perr := parser.ParseFile(token.NewFileSet(), path, nil, 0)
			require.NoError(t, perr, "parsing %s", path)
			ast.Inspect(f, func(n ast.Node) bool {
				if lit, ok := n.(*ast.BasicLit); ok && lit.Kind == token.STRING {
					v := lit.Value
					if u, uerr := strconv.Unquote(v); uerr == nil {
						v = u
					}
					for _, m := range required.FindAllStringSubmatch(v, -1) {
						found[m[1]] = true
					}
				}
				return true
			})
			return nil
		default:
			return nil
		}

		b, rerr := os.ReadFile(path)
		require.NoError(t, rerr)
		dec := yaml.NewDecoder(strings.NewReader(string(b)))
		for {
			var doc any
			if derr := dec.Decode(&doc); derr != nil {
				require.ErrorIs(t, derr, io.EOF, "parsing %s", path)
				return nil
			}
			walkStrings(doc, func(v string) {
				for _, m := range required.FindAllStringSubmatch(v, -1) {
					found[m[1]] = true
				}
			})
		}
	}))

	for v := range built {
		require.True(t, found[v],
			"%s is built here but is not a required compose interpolation anywhere; "+
				"a `${%s:-default}` form silently boots a published image when the "+
				"caller forgets an override", v, v)
	}
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
