package snapshot

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// The compose files declare in-repo images as required, so a CLI-path call
// that loses its env files fails to interpolate rather than falling back to a
// published image. These pin the two properties that keep that working.
func TestComposeArgs(t *testing.T) {
	write := func(t *testing.T, dir string, names ...string) {
		t.Helper()
		for _, n := range names {
			require.NoError(t, os.WriteFile(filepath.Join(dir, n), nil, 0644))
		}
	}

	t.Run("passes both, published first so .env still wins", func(t *testing.T) {
		dir := t.TempDir()
		write(t, dir, ".env.published", ".env")
		require.Equal(t, []string{
			"compose",
			"--env-file", ".env.published",
			"--env-file", ".env",
			"ps", "--all",
		}, composeArgs(dir, "ps", "--all"))
	})

	// `--env-file` on a missing path is a hard error, unlike compose's
	// implicit .env, so a project without one must not gain a broken flag.
	t.Run("skips whichever is absent", func(t *testing.T) {
		dir := t.TempDir()
		write(t, dir, ".env")
		require.Equal(t,
			[]string{"compose", "--env-file", ".env", "stop"},
			composeArgs(dir, "stop"))

		require.Equal(t,
			[]string{"compose", "stop"},
			composeArgs(t.TempDir(), "stop"))
	})
}
