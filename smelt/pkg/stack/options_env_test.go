package stack

import "testing"

// imageFields pairs each override variable with the config field it must
// land in. Written out independently of envImageOptions on purpose: a
// hand-maintained table of (string, func) pairs invites a copy-paste slip
// that points two variables at the same option, and only a second, separate
// statement of the mapping can catch it.
var imageFields = []struct {
	env   string
	value string
	get   func(*config) string
}{
	{"PIRI_IMAGE", "img-piri", func(c *config) string { return c.piriImage }},
	{"GUPPY_IMAGE", "img-guppy", func(c *config) string { return c.guppyImage }},
	{"INDEXER_IMAGE", "img-indexer", func(c *config) string { return c.indexerImage }},
	{"DELEGATOR_IMAGE", "img-delegator", func(c *config) string { return c.delegatorImage }},
	{"UPLOAD_IMAGE", "img-upload", func(c *config) string { return c.uploadImage }},
	{"HILT_IMAGE", "img-hilt", func(c *config) string { return c.hiltImage }},
	{"SIGNER_IMAGE", "img-signer", func(c *config) string { return c.signerImage }},
	{"BLOCKCHAIN_IMAGE", "img-blockchain", func(c *config) string { return c.blockchainImage }},
	{"IPNI_IMAGE", "img-ipni", func(c *config) string { return c.ipniImage }},
	{"INGOT_IMAGE", "img-ingot", func(c *config) string { return c.ingotImage }},
	{"SWARF_IMAGE", "img-swarf", func(c *config) string { return c.swarfImage }},
}

// clearEnv blanks every variable OptionsFromEnv reads, so a test does not
// inherit the developer's or the CI job's own overrides.
func clearEnv(t *testing.T) {
	t.Helper()
	for _, f := range imageFields {
		t.Setenv(f.env, "")
	}
	t.Setenv("SMELT_WORKSPACE", "")
}

func apply(opts []Option) *config {
	c := &config{}
	for _, o := range opts {
		o(c)
	}
	return c
}

func TestOptionsFromEnvEmpty(t *testing.T) {
	clearEnv(t)
	if opts := OptionsFromEnv(); len(opts) != 0 {
		t.Fatalf("expected no options from a blank environment, got %d", len(opts))
	}
}

// Every variable must reach its own field. Catches both a missing entry and
// two entries wired to the same option.
func TestOptionsFromEnvEachImageReachesItsField(t *testing.T) {
	clearEnv(t)
	for _, f := range imageFields {
		t.Setenv(f.env, f.value)
	}
	c := apply(OptionsFromEnv())
	for _, f := range imageFields {
		if got := f.get(c); got != f.value {
			t.Errorf("%s: config field = %q, want %q", f.env, got, f.value)
		}
	}
}

func TestOptionsFromEnvWorkspace(t *testing.T) {
	clearEnv(t)
	if apply(OptionsFromEnv()).workspaceBinaries {
		t.Error("workspaceBinaries set without SMELT_WORKSPACE")
	}
	t.Setenv("SMELT_WORKSPACE", "1")
	if !apply(OptionsFromEnv()).workspaceBinaries {
		t.Error("SMELT_WORKSPACE set but workspaceBinaries is false")
	}
}

// Appending the env options after a test's own must let the environment win.
func TestOptionsFromEnvOverridesHardcoded(t *testing.T) {
	clearEnv(t)
	t.Setenv("PIRI_IMAGE", "img-from-env")
	opts := append([]Option{WithPiriImage("img-hardcoded")}, OptionsFromEnv()...)
	if got := apply(opts).piriImage; got != "img-from-env" {
		t.Errorf("piriImage = %q, want the environment's value", got)
	}
}
