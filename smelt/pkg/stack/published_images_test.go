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

// walkYAMLStrings visits every string scalar VALUE in a decoded document.
//
// KEYS ARE NOT VISITED. An earlier revision visited them, justified by
// "compose interpolates throughout, not only in values" -- which is the
// opposite of what compose does. Measured with the client: a `${X:?}` in an
// `environment` key, a `labels` key or a top-level `x-` key leaves
// `docker compose config` exiting 0 with the variable unset, and the output
// re-escapes the `$` to `$$` -- compose saying it treated the text as a
// literal. Visiting keys put back exactly the over-reading the parse rewrite
// was written to remove.
//
// An earlier revision named a SERVICE NAME as a fourth such position. It is
// not one: compose exits 1 there with "services additional properties '...'
// not allowed", schema validation refusing the name before interpolation is
// reached. The conclusion held; one of the four cases cited had not been run.
func walkYAMLStrings(n any, fn func(string)) {
	switch v := n.(type) {
	case string:
		fn(v)
	case []any:
		for _, e := range v {
			walkYAMLStrings(e, fn)
		}
	case map[string]any:
		for _, e := range v {
			walkYAMLStrings(e, fn)
		}
	case map[any]any:
		for _, e := range v {
			walkYAMLStrings(e, fn)
		}
	}
}

// composeFileNames are the names `docker compose` discovers on its own in a
// directory it is run from.
//
// THE OVERRIDE NAMES COUNT. An earlier revision listed only the four base
// names and called that the complete auto-discovery list. Compose also loads
// `compose.override.{yml,yaml}` and `docker-compose.override.{yml,yaml}`
// alongside them, and a `${X:?}` in one is required exactly as in the base
// file -- verified with the client, rc=1 naming the variable. That is the live
// `make up` path: smelt's COMPOSE passes no `-f` unless the workspace override
// exists, and `-f` is what suppresses auto-discovery.
var composeFileNames = map[string]bool{
	"compose.yaml": true, "compose.yml": true,
	"compose.override.yaml": true, "compose.override.yml": true,
	"docker-compose.yaml": true, "docker-compose.yml": true,
	"docker-compose.override.yaml": true, "docker-compose.override.yml": true,
}

// includePaths returns the `include:` targets a compose document names,
// resolved against its own directory. Entries are a path, a list of paths, or
// a mapping with `path:` holding either.
func includePaths(dir string, doc any) []string {
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

// extendsPaths returns the files a document's services pull in with
// `extends: {file: ...}`. A SECOND WAY COMPOSE READS A FILE, which an earlier
// revision's comment denied by saying it "reaches anything else only through
// an include:". Verified with the client: a required variable in an
// extends.file target is required. Nothing in this tree uses it today; it is
// covered because the comment claimed totality and the claim was wrong.
func extendsPaths(dir string, doc any) []string {
	m, ok := doc.(map[string]any)
	if !ok {
		return nil
	}
	services, ok := m["services"].(map[string]any)
	if !ok {
		return nil
	}
	var out []string
	for _, svc := range services {
		sm, ok := svc.(map[string]any)
		if !ok {
			continue
		}
		ext, ok := sm["extends"].(map[string]any)
		if !ok {
			continue
		}
		if f, ok := ext["file"].(string); ok {
			out = append(out, filepath.Join(dir, f))
		}
	}
	return out
}

// requiredImageVars returns the image variables compose requires, from the
// files compose actually reads.
//
// THE FILE SET IS WHAT COMPOSE READS, not every YAML under the root. Compose
// discovers `compose.yml`/`compose.yaml` (and the `docker-compose` spellings)
// in a directory it is run from -- the root for `make up`, and each
// systems/<x>/ for the standalone runs the root file documents -- and reaches
// anything else only through an `include:`. So the set is those, plus their
// include targets transitively.
//
// AN EARLIER REVISION SCANNED EVERY .yml AND .yaml, on the grounds that
// over-reading was the safe direction. It is not, and the same function said
// so a few lines down: `wantVars` comes from the two Go tables and never from
// what the YAML requires, so a match on text compose does not interpolate
// keeps a name alive in `required` and hides the drift. Under a blanket scan,
// a `${HILT_IMAGE:?...}` written into systems/telemetry/config/prometheus.yml
// -- a file compose never opens -- would mask a hilt image pinned literally.
//
// ROOTED AT THE SMELT ROOT, not at systems/: smelt/compose.yml is the file
// `make up` resolves and it sits outside systems/.
//
// TWO KNOWN LIMITS, stated because the comment above would otherwise read as
// total and rule 5 is about exactly that. `include:` accepts a
// `project_directory:` that redirects how the included file's OWN nested
// includes resolve, and this reads them relative to the included file instead.
// And `include.env_file:` can supply a required variable, which this does not
// read -- that one errs towards a false failure. Neither appears in this tree;
// both are left uncovered rather than half-covered.
func requiredImageVars(t *testing.T, root string) map[string]bool {
	t.Helper()
	// `:?` and `?` are BOTH required forms. `${X:?msg}` errors when X is unset
	// or empty; `${X?msg}` errors only when unset. Compose rejects either the
	// same way -- "required variable NEWTHING_IMAGE is missing a value" --
	// and an earlier version of this pattern matched only the first, so the
	// other spelling of the same line left this test green.
	re := regexp.MustCompile(`\$\{([A-Z0-9_]+_IMAGE):?\?`)
	vars := map[string]bool{}

	match := func(v string) {
		// `$$` is compose's escape for a literal `$`. Blank it out first so
		// `$${X:?}` cannot match; a lone `$` is untouched. Left-to-right
		// pairing matches compose's own lexer, checked against the client for
		// runs of one to five dollars.
		for _, m := range re.FindAllStringSubmatch(strings.ReplaceAll(v, "$$", "\x00\x00"), -1) {
			vars[m[1]] = true
		}
	}

	// PARSED, NOT GREPPED OVER THE RAW BYTES, and the difference is not
	// tidiness: a raw scan matches inside a `#` comment, so commenting out an
	// interpolation while pinning the image literally left this test green on
	// an .env.published entry nothing needed.
	parse := func(path string) []any {
		b, err := os.ReadFile(path)
		require.NoError(t, err)
		var docs []any
		dec := yaml.NewDecoder(strings.NewReader(string(b)))
		for {
			var doc any
			if err := dec.Decode(&doc); err != nil {
				// Anything but a clean end of stream means a file compose
				// reads does not parse, which is worth failing on rather than
				// falling back to a raw scan and its comment problem.
				require.ErrorIs(t, err, io.EOF, "parsing %s", path)
				return docs
			}
			docs = append(docs, doc)
		}
	}

	// Seed with every auto-discoverable name under the root, then follow
	// include: to a fixpoint.
	seen := map[string]bool{}
	var queue []string
	require.NoError(t, filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && composeFileNames[d.Name()] {
			queue = append(queue, path)
		}
		return nil
	}))
	require.NotEmpty(t, queue, "found no compose file under %s", root)

	for len(queue) > 0 {
		path := queue[0]
		queue = queue[1:]
		abs, err := filepath.Abs(path)
		require.NoError(t, err)
		if seen[abs] {
			continue
		}
		seen[abs] = true

		for _, doc := range parse(path) {
			walkYAMLStrings(doc, match)
			for _, inc := range append(includePaths(filepath.Dir(path), doc),
				extendsPaths(filepath.Dir(path), doc)...) {
				if _, err := os.Stat(inc); err == nil {
					queue = append(queue, inc)
					continue
				}
				// A MISSING TARGET IS FATAL, with one named exception. An
				// earlier revision skipped any absent path "because that is
				// compose's error to report" -- but compose reports it by
				// failing the whole project (rc=1, "no such file or
				// directory"), while this test went green having quietly
				// dropped every variable that file named. Mistyping
				// systems/swarf as systems/swraf in the root include loses
				// SWARF_IMAGE from `required` and passes.
				//
				// The exception is generated/, which is gitignored build
				// output from `go run ./cmd/smelt generate`; the root include
				// names generated/compose/piri.yml and it is absent in a clean
				// checkout. That is the only absence this skip was ever for,
				// which the earlier comment did not say, so nobody could
				// tighten it safely.
				rel, rerr := filepath.Rel(root, inc)
				require.NoError(t, rerr)
				require.True(t, strings.HasPrefix(rel, "generated"+string(filepath.Separator)),
					"%s includes %s, which does not exist; compose fails the whole project on that",
					path, inc)
			}
		}
	}

	// PIRI_IMAGE appears in no compose file at all -- pkg/generate emits piri's
	// service definition -- so the generator is a second source and a scan of
	// compose files alone reports seven of eight.
	//
	// ITS TESTS ARE EXCLUDED. Nothing in a _test.go reaches compose, and
	// generate_test.go already asserts on emitted interpolations verbatim
	// ("${SMELT_PIRI_0_PORT:-15100:3000}"); the same assertion for the image
	// line is the obvious next test to write, and would pin PIRI_IMAGE in
	// `required` for good.
	//
	// The generator is Go, so read its STRING LITERALS rather than its bytes --
	// symmetric with parsing the YAML, and for the same reason: a commented-out
	// interpolation would otherwise mask the same drift.
	gen := filepath.Join(root, "pkg", "generate")
	require.NoError(t, filepath.WalkDir(gen, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		f, perr := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		require.NoError(t, perr, "parsing %s", path)
		ast.Inspect(f, func(n ast.Node) bool {
			if lit, ok := n.(*ast.BasicLit); ok && lit.Kind == token.STRING {
				if v, uerr := strconv.Unquote(lit.Value); uerr == nil {
					match(v)
				} else {
					match(lit.Value)
				}
			}
			return true
		})
		return nil
	}))

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
