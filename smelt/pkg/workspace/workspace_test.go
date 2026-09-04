package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRenderOverrideBinariesAndConfigs(t *testing.T) {
	data, err := RenderOverride(
		map[string]string{"ingot": "/host/bin/ingot", "piri": "/host/bin/piri"},
		map[string]string{"ingot": "/host/cfg/config.yaml"},
		[]string{"piri-0", "piri-1"},
	)
	if err != nil {
		t.Fatalf("RenderOverride: %v", err)
	}
	out := string(data)

	for _, want := range []string{
		"/host/bin/ingot:/usr/bin/ingot:ro",
		"/host/cfg/config.yaml:/etc/ingot/config.yaml:ro",
		"piri-0:",
		"piri-1:",
		"/host/bin/piri:/usr/bin/piri:ro",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("override missing %q:\n%s", want, out)
		}
	}
}

func TestRenderOverrideUnknownService(t *testing.T) {
	if _, err := RenderOverride(map[string]string{"nope": "/x"}, nil, nil); err == nil {
		t.Fatal("expected error for unknown binary service")
	}
	if _, err := RenderOverride(nil, map[string]string{"nope": "/x"}, nil); err == nil {
		t.Fatal("expected error for unknown config service")
	}
}

func TestRenderOverrideNoConfigPath(t *testing.T) {
	// guppy has no registered configPath — config override must error, not
	// silently mount nowhere.
	if _, err := RenderOverride(nil, map[string]string{"guppy": "/x"}, nil); err == nil {
		t.Fatal("expected error for service without a config path")
	}
}

// piri does not compile CGO-free without the skiff build tag: it selects
// Curio's FFI-free variants, and without it the build dies on undefined ffi.*
// symbols in curio/lib/ffiselect, harmony/resources/ffigpu and lib/paths.
//
// This guard exists because the omission was real and survived in three
// separate places at once — piri's .goreleaser.yaml, this builder, and
// (correctly, always) piri's Makefile, which was the only one passing it. The
// failure mode is bad: workspace mode simply could not build piri, and the
// error surfaces as a wall of unrelated-looking Curio symbol errors.
func TestPiriCarriesSkiffTag(t *testing.T) {
	spec, ok := Services["piri"]
	if !ok {
		t.Fatal("piri missing from Services")
	}
	if !strings.Contains(spec.buildTags, "skiff") {
		t.Fatalf("piri must build with the skiff tag, got buildTags=%q", spec.buildTags)
	}
}

// TestDetectSelectsOnlyWorkspaceModules pins the rule that Detect rebuilds
// exactly the services whose module is in the use-list. Shared modules
// (libforge, commands, internal) must not widen the set: a workspace listing
// them plus a subset of the services — the monorepo's go.work — used to select
// all eight services and fail BuildBinary on module dirs that do not exist.
func TestDetectSelectsOnlyWorkspaceModules(t *testing.T) {
	dir := t.TempDir()
	gowork := filepath.Join(dir, "go.work")
	content := "go 1.27.0\n\nuse (\n\t./commands\n\t./hilt\n\t./internal\n\t./libforge\n\t./smelt\n\t./smelt/systems/stress-tester\n\t./sprue\n)\n"
	if err := os.WriteFile(gowork, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOWORK", gowork)

	root, services, err := Detect()
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if root != dir {
		t.Errorf("root = %q, want %q", root, dir)
	}
	if got, want := strings.Join(services, ","), "hilt,upload"; got != want {
		t.Errorf("services = %q, want %q (sprue's compose name is upload; shared modules must not add services)", got, want)
	}
}
