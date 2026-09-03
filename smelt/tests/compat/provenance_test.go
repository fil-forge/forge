//go:build compat

package compat

import (
	"context"
	"fmt"
	"path"
	"strings"
	"testing"

	"github.com/fil-forge/forge/smelt/pkg/stack"
)

// These tests need no Docker daemon: they drive the provenance guard from a
// fake inspector built on the same stack.ContainerInfo the real Stack.Inspect
// returns. Run them alone with
//
//	go test -tags compat -run 'TestCheckProvenance|TestDescribe' ./tests/compat
//
// (TestMain's leaked-container sweep logs a warning without a daemon and
// carries on.)

// fakeStack is an inspector over canned inspect records.
type fakeStack struct {
	piri   []string
	byName map[string]*stack.ContainerInfo
}

func (f *fakeStack) Inspect(_ context.Context, name string) (*stack.ContainerInfo, error) {
	info, ok := f.byName[name]
	if !ok {
		return nil, fmt.Errorf("no container for service %q", name)
	}
	return info, nil
}

func (f *fakeStack) PiriServiceNames() []string { return f.piri }

// running fakes `docker inspect` for a container created from image with a
// read-only bind mount at each of mounts — the shape pkg/workspace's override
// produces.
func running(image string, mounts ...string) *stack.ContainerInfo {
	info := &stack.ContainerInfo{Image: image}
	for _, m := range mounts {
		info.Mounts = append(info.Mounts, stack.Mount{
			Type:        "bind",
			Source:      "/tmp/smelt-build/" + path.Base(m),
			Destination: m,
			ReadOnly:    true,
		})
	}
	return info
}

func mustRow(t *testing.T, compose string) service {
	t.Helper()
	r, ok := row(compose)
	if !ok {
		t.Fatalf("service table has no %q row; update this test's row bindings", compose)
	}
	return r
}

func TestCheckProvenance(t *testing.T) {
	piri, ingot, upload, hilt := mustRow(t, "piri"), mustRow(t, "ingot"), mustRow(t, "upload"), mustRow(t, "hilt")
	old := piri.ref("sha-96a672e")
	base := map[string]string{
		"piri":   piri.ref("sha-f60dd59"),
		"ingot":  ingot.ref("sha-f60dd59"),
		"upload": upload.ref("sha-f60dd59"),
		"hilt":   hilt.ref("sha-f60dd59"),
	}
	floating := func(svc service) string { return svc.ref("main") }

	cases := []struct {
		name  string
		stack *fakeStack
		want  provenance
		// wantErr lists substrings the joined error must contain; empty
		// means the check must pass. mustNotMention are container names
		// that must NOT appear (the guard names only the wrong containers).
		// errLines is the exact number of mismatches expected.
		wantErr        []string
		mustNotMention []string
		errLines       int
	}{
		{
			name: "pinned peer, correct",
			stack: &fakeStack{piri: []string{"piri-0"}, byName: map[string]*stack.ContainerInfo{
				"piri-0": running(old),
				"ingot":  running(floating(ingot), "/usr/bin/ingot"),
				"upload": running(floating(upload), "/usr/bin/sprue"),
				"hilt":   running(floating(hilt), "/usr/bin/hilt"),
			}},
			want: provenance{"piri": old},
		},
		{
			// The vacuous case the guard exists for: the exclusion was
			// missing, so HEAD piri is mounted over the pinned image.
			name: "pinned peer, HEAD binary mounted over the pin",
			stack: &fakeStack{piri: []string{"piri-0"}, byName: map[string]*stack.ContainerInfo{
				"piri-0": running(old, "/usr/bin/piri"),
				"ingot":  running(floating(ingot), "/usr/bin/ingot"),
				"upload": running(floating(upload), "/usr/bin/sprue"),
				"hilt":   running(floating(hilt), "/usr/bin/hilt"),
			}},
			want:           provenance{"piri": old},
			wantErr:        []string{"piri-0", old, "/usr/bin/piri", "HEAD is what runs"},
			mustNotMention: []string{"ingot:", "upload:", "hilt:"},
			errLines:       1,
		},
		{
			name: "pinned peer, container created from a different image",
			stack: &fakeStack{piri: []string{"piri-0"}, byName: map[string]*stack.ContainerInfo{
				"piri-0": running(floating(piri)),
				"ingot":  running(floating(ingot), "/usr/bin/ingot"),
				"upload": running(floating(upload), "/usr/bin/sprue"),
				"hilt":   running(floating(hilt), "/usr/bin/hilt"),
			}},
			want:     provenance{"piri": old},
			wantErr:  []string{"piri-0", "pinned to " + old, `created from "` + floating(piri) + `"`},
			errLines: 1,
		},
		{
			// Wrong image AND a mount: both are reported for the one container.
			name: "pinned peer, wrong image with a HEAD mount on top",
			stack: &fakeStack{piri: []string{"piri-0"}, byName: map[string]*stack.ContainerInfo{
				"piri-0": running(floating(piri), "/usr/bin/piri"),
				"ingot":  running(floating(ingot), "/usr/bin/ingot"),
				"upload": running(floating(upload), "/usr/bin/sprue"),
				"hilt":   running(floating(hilt), "/usr/bin/hilt"),
			}},
			want:     provenance{"piri": old},
			wantErr:  []string{"created from", "HEAD is what runs"},
			errLines: 2,
		},
		{
			// The other half of the hazard: a "HEAD" service that lost its
			// mount is a floating :main image, not this commit.
			name: "pinned peer, a HEAD service is not mounted",
			stack: &fakeStack{piri: []string{"piri-0"}, byName: map[string]*stack.ContainerInfo{
				"piri-0": running(old),
				"ingot":  running(floating(ingot), "/usr/bin/ingot"),
				"upload": running(floating(upload), "/usr/bin/sprue"),
				"hilt":   running(floating(hilt)),
			}},
			want:           provenance{"piri": old},
			wantErr:        []string{"hilt:", "/usr/bin/hilt", "not under test"},
			mustNotMention: []string{"piri-0", "ingot:", "upload:"},
			errLines:       1,
		},
		{
			// A volume at the binary path is not a host-built binary.
			name: "pinned peer, a HEAD service has a volume, not a bind mount, at its binary",
			stack: &fakeStack{piri: []string{"piri-0"}, byName: map[string]*stack.ContainerInfo{
				"piri-0": running(old),
				"ingot":  running(floating(ingot), "/usr/bin/ingot"),
				"upload": running(floating(upload), "/usr/bin/sprue"),
				"hilt": {Image: floating(hilt), Mounts: []stack.Mount{
					{Type: "volume", Source: "/var/lib/docker/volumes/x/_data", Destination: "/usr/bin/hilt"},
				}},
			}},
			want:     provenance{"piri": old},
			wantErr:  []string{"hilt:", "not under test"},
			errLines: 1,
		},
		{
			name: "rolling upgrade of piri, correct, two nodes",
			stack: &fakeStack{piri: []string{"piri-0", "piri-1"}, byName: map[string]*stack.ContainerInfo{
				"piri-0": running(base["piri"], "/usr/bin/piri"),
				"piri-1": running(base["piri"], "/usr/bin/piri"),
				"ingot":  running(base["ingot"]),
				"upload": running(base["upload"]),
				"hilt":   running(base["hilt"]),
			}},
			want: provenance{"ingot": base["ingot"], "upload": base["upload"], "hilt": base["hilt"]},
		},
		{
			name: "rolling upgrade of piri, second node never got the HEAD binary",
			stack: &fakeStack{piri: []string{"piri-0", "piri-1"}, byName: map[string]*stack.ContainerInfo{
				"piri-0": running(base["piri"], "/usr/bin/piri"),
				"piri-1": running(base["piri"]),
				"ingot":  running(base["ingot"]),
				"upload": running(base["upload"]),
				"hilt":   running(base["hilt"]),
			}},
			want:           provenance{"ingot": base["ingot"], "upload": base["upload"], "hilt": base["hilt"]},
			wantErr:        []string{"piri-1", "/usr/bin/piri", "not under test"},
			mustNotMention: []string{"piri-0"},
			errLines:       1,
		},
		{
			// What the old otherThan seam could have produced: an exclusion
			// spelled "sprue" holds back nothing, so HEAD sprue is mounted
			// over the baseline image the test believes it pinned.
			name: "rolling upgrade of ingot, sprue was not really held back",
			stack: &fakeStack{piri: []string{"piri-0"}, byName: map[string]*stack.ContainerInfo{
				"piri-0": running(base["piri"]),
				"ingot":  running(base["ingot"], "/usr/bin/ingot"),
				"upload": running(base["upload"], "/usr/bin/sprue"),
				"hilt":   running(base["hilt"]),
			}},
			want:     provenance{"piri": base["piri"], "upload": base["upload"], "hilt": base["hilt"]},
			wantErr:  []string{"upload:", "/usr/bin/sprue", "HEAD is what runs"},
			errLines: 1,
		},
		{
			name: "every container wrong reports every container",
			stack: &fakeStack{piri: []string{"piri-0"}, byName: map[string]*stack.ContainerInfo{
				"piri-0": running(old, "/usr/bin/piri"),
				"ingot":  running(floating(ingot)),
				"upload": running(floating(upload)),
				"hilt":   running(floating(hilt)),
			}},
			want:     provenance{"piri": old},
			wantErr:  []string{"piri-0", "ingot:", "upload:", "hilt:"},
			errLines: 4,
		},
		{
			name: "a container that cannot be inspected is an error, not a pass",
			stack: &fakeStack{piri: []string{"piri-0"}, byName: map[string]*stack.ContainerInfo{
				"piri-0": running(old),
				"ingot":  running(floating(ingot), "/usr/bin/ingot"),
				"hilt":   running(floating(hilt), "/usr/bin/hilt"),
			}},
			want:     provenance{"piri": old},
			wantErr:  []string{"upload", "no container"},
			errLines: 1,
		},
		{
			name: "a stack with no piri nodes is an error, not a pass",
			stack: &fakeStack{piri: nil, byName: map[string]*stack.ContainerInfo{
				"ingot":  running(floating(ingot), "/usr/bin/ingot"),
				"upload": running(floating(upload), "/usr/bin/sprue"),
				"hilt":   running(floating(hilt), "/usr/bin/hilt"),
			}},
			want:     provenance{"piri": old},
			wantErr:  []string{"no piri nodes"},
			errLines: 1,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := checkProvenance(context.Background(), tc.stack, tc.want)
			if len(tc.wantErr) == 0 {
				if err != nil {
					t.Fatalf("expected the stack to pass provenance, got:\n%v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected the guard to fire, it passed %s", describe(tc.want))
			}
			msg := err.Error()
			for _, want := range tc.wantErr {
				if !strings.Contains(msg, want) {
					t.Errorf("error should mention %q:\n%s", want, msg)
				}
			}
			for _, not := range tc.mustNotMention {
				if strings.Contains(msg, not) {
					t.Errorf("error should not mention %q (that container was fine):\n%s", not, msg)
				}
			}
			if got := len(strings.Split(msg, "\n")); tc.errLines > 0 && got != tc.errLines {
				t.Errorf("expected %d mismatch(es), got %d:\n%s", tc.errLines, got, msg)
			}
			t.Logf("guard fired:\n%s", msg)
		})
	}
}

// TestDescribe pins the format of the `compat: exercised` line the workflow
// counts and prints: one line, sorted, greppable.
func TestDescribe(t *testing.T) {
	old := mustRow(t, "piri").ref("sha-96a672e")
	got := describe(provenance{"piri": old})
	want := "pinned=piri@" + old + " head=hilt,ingot,upload"
	if got != want {
		t.Errorf("describe = %q, want %q", got, want)
	}
	if strings.Contains(got, "\n") {
		t.Errorf("describe must be a single line: %q", got)
	}
}

// TestBindMountAt pins the one predicate the guard rests on: only a bind
// mount at exactly the binary path counts.
func TestBindMountAt(t *testing.T) {
	info := running("img", "/usr/bin/piri")
	info.Mounts = append(info.Mounts, stack.Mount{Type: "volume", Destination: "/data"})
	if _, ok := info.BindMountAt("/usr/bin/piri"); !ok {
		t.Error("expected the bind mount at /usr/bin/piri to be found")
	}
	if _, ok := info.BindMountAt("/usr/bin"); ok {
		t.Error("a parent directory must not match")
	}
	if _, ok := info.BindMountAt("/data"); ok {
		t.Error("a volume must not count as a bind mount")
	}
}
