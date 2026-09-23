package stack

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// The published image set is stated in four places, and they must agree:
//
//   - the compose files, which require each one (`${HILT_IMAGE:?...}`) rather
//     than defaulting it, so a caller who forgets an override fails loudly;
//   - `.env.published`, which supplies them to `make up`;
//   - publishedImages, which supplies them to WithPublishedImages();
//   - envImageOptions, which lets a Go caller override them by the same names.
//
// They had already drifted: indexing-service was added to publishedImages and
// not to .env.published, so `make up` failed at `compose up` with INDEXER_IMAGE
// unset, for as long as the indexer had been in the stack. Three comments in
// this repository asserted that could not happen silently, on the grounds that
// a missing entry fails at `compose up`. It does — and nothing in CI runs
// `make up`, so "not silent" and "not noticed" were the same thing.
//
// WHY THIS IS A GO TEST AND NOT A check-*.sh. The first version was a shell
// script, and it compared the lists as SETS: variable names in one comparison,
// image references in the other, never the pairing between them. It reported
// agreement, and exited 0, on a tree where PIRI_IMAGE and HILT_IMAGE had been
// swapped in .env.published -- each service booting the other's image. Closing
// that in shell means recovering which config field each variable and each
// reference reaches, which is Go semantics, so it belongs here. Both tables are
// already data in this package; the test reads them rather than parsing them.

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

// requiredImageVars walks the smelt root and returns the required image
// variables its YAML and its compose generator name.
//
// ROOTED AT THE SMELT ROOT, not at systems/. smelt/compose.yml is the file
// `make up` resolves and it sits outside systems/; a walk rooted one directory
// deeper would let a required variable added there go unchecked. That is the
// same bug class this test exists for, at the one compose file systems/ does
// not contain.
//
// EVERY .yml AND .yaml, not files named compose.yml. The root file pulls in
// its parts with `include:`, whose paths it names explicitly, so an included
// file called anything else would be missed -- and `compose.yaml` is compose's
// own auto-discovered default. Over-reading is the safe direction here and
// costs nothing measurable: across every YAML file under this root, the
// pattern matches only compose files.
func requiredImageVars(t *testing.T, root string) map[string]bool {
	t.Helper()
	// `:?` and `?` are BOTH required forms. `${X:?msg}` errors when X is unset
	// or empty; `${X?msg}` errors only when unset. Compose rejects either the
	// same way -- "required variable NEWTHING_IMAGE is missing a value" --
	// and an earlier version of this pattern matched only the first, so the
	// other spelling of the same line left this test green.
	re := regexp.MustCompile(`\$\{([A-Z0-9_]+_IMAGE):?\?`)
	vars := map[string]bool{}

	scan := func(path string) {
		b, err := os.ReadFile(path)
		require.NoError(t, err)
		for _, m := range re.FindAllStringSubmatch(string(b), -1) {
			vars[m[1]] = true
		}
	}

	composeFiles := 0
	require.NoError(t, filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		// PIRI_IMAGE appears in no compose file at all -- pkg/generate emits
		// piri's service definition -- so the generator is a second source and
		// a glob over compose files alone reports seven of eight.
		switch {
		case strings.HasSuffix(path, ".yml"), strings.HasSuffix(path, ".yaml"):
			composeFiles++
			scan(path)
		case strings.HasPrefix(path, filepath.Join(root, "pkg", "generate")) &&
			strings.HasSuffix(path, ".go"):
			scan(path)
		}
		return nil
	}))

	require.NotZero(t, composeFiles, "found no YAML under %s", root)
	require.NotEmpty(t, vars, "found no required ${X_IMAGE:?} interpolations")
	return vars
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
		// comparison: the earlier shell version passed on a file with
		// PIRI_IMAGE bound twice to different images.
		_, dup := out[k]
		require.False(t, dup, "%s:%d: %s is set more than once", path, i+1, k)
		out[k] = v
	}
	return out
}

func TestPublishedImageSetAgrees(t *testing.T) {
	root := filepath.Join("..", "..")

	// Pair each compose variable and each published reference to the config
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

	required := requiredImageVars(t, root)
	env := readEnvPublished(t, filepath.Join(root, ".env.published"))

	// 1. Every required compose variable must be overridable from Go, or a
	//    caller can set it for `docker compose` and not for a Go test.
	for v := range required {
		require.Contains(t, varToField, v,
			"%s is a required compose interpolation but is not in envImageOptions", v)
	}

	// 2. Required, supplied by .env.published, and present in publishedImages
	//    must be the same set -- and the REFERENCE each names must match,
	//    which is what a set comparison cannot see.
	var wantVars []string
	for v, f := range varToField {
		if _, built := fieldToRef[f]; built {
			wantVars = append(wantVars, v)
		}
	}
	sort.Strings(wantVars)

	var gotRequired, gotEnv []string
	for v := range required {
		gotRequired = append(gotRequired, v)
	}
	for v := range env {
		gotEnv = append(gotEnv, v)
	}
	sort.Strings(gotRequired)
	sort.Strings(gotEnv)

	require.Equal(t, wantVars, gotRequired,
		"the compose files and publishedImages disagree about which images this repository builds")
	require.Equal(t, wantVars, gotEnv,
		".env.published does not supply exactly the required images")

	for _, v := range wantVars {
		require.Equal(t, fieldToRef[varToField[v]], env[v],
			"%s in .env.published names a different image than publishedImages does", v)
	}
}
