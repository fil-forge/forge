package workspace

import (
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

func TestRenderOverrideRegistrarFanOut(t *testing.T) {
	// upload and hilt binaries must also be mounted into their one-shot
	// registrar services, which run the same image's CLI — otherwise a
	// workspace build would test a local server against the published CLI.
	data, err := RenderOverride(
		map[string]string{"upload": "/host/bin/sprue", "hilt": "/host/bin/hilt"},
		nil, nil,
	)
	if err != nil {
		t.Fatalf("RenderOverride: %v", err)
	}
	out := string(data)

	for _, want := range []string{
		"upload-init:",
		"hilt-init:",
		"/host/bin/sprue:/usr/bin/sprue:ro",
		"/host/bin/hilt:/usr/bin/hilt:ro",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("override missing %q:\n%s", want, out)
		}
	}
	if got := strings.Count(out, "/host/bin/sprue:/usr/bin/sprue:ro"); got != 2 {
		t.Errorf("sprue binary mounted %d time(s), want 2 (upload + upload-init):\n%s", got, out)
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

// TestPiriBuildsWithSkiff pins the tag that piri cannot build without. Dropping
// it does not fail here in any obvious way — it fails much later, as undefined
// ffi.*/supraffi.* symbols inside curio, from a build path nobody was looking
// at. That has now happened four times: piri's .goreleaser.yaml, this
// workspace builder, the repo CI, and the monorepo's e2e job.
func TestPiriBuildsWithSkiff(t *testing.T) {
	spec, ok := Services["piri"]
	if !ok {
		t.Fatal("no piri entry in Services")
	}
	if spec.buildTags != "skiff" {
		t.Fatalf("piri buildTags = %q, want \"skiff\"", spec.buildTags)
	}
	args := buildArgs(spec, "/tmp/piri")
	var sawTags bool
	for i, a := range args {
		if a == "-tags" {
			if i+1 >= len(args) || args[i+1] != "skiff" {
				t.Fatalf("-tags not followed by skiff: %v", args)
			}
			sawTags = true
		}
	}
	if !sawTags {
		t.Fatalf("buildArgs dropped the tag: %v", args)
	}
}

// TestBuildArgsOmitsEmptyTags keeps the other direction honest: a service with
// no tags must not get a bare -tags flag, which go build rejects.
func TestBuildArgsOmitsEmptyTags(t *testing.T) {
	args := buildArgs(serviceBuild{buildTarget: "./cmd"}, "/tmp/x")
	for _, a := range args {
		if a == "-tags" {
			t.Fatalf("unexpected -tags for a service with none: %v", args)
		}
	}
}
