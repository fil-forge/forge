//go:build compat

package compat

import (
	"sort"
	"strings"
	"testing"

	"github.com/fil-forge/forge/smelt/pkg/workspace"
)

// TestServiceTable is what makes the table load-bearing. It needs no Docker
// daemon, so it runs wherever the package compiles:
//
//	go test -tags compat -run TestServiceTable ./tests/compat
//
// A row naming a compose service pkg/workspace cannot build would make its
// exclusion a no-op (pkg/stack now refuses that too) and leave the provenance
// check no binary path to look at; and, with an active go.work as in the compat
// job, the set of services the workspace WOULD build from HEAD must be exactly
// this table — a service built but not in the table would run unasserted, a
// service in the table but not built would never be HEAD.
func TestServiceTable(t *testing.T) {
	seenCompose, seenImage := map[string]bool{}, map[string]bool{}
	for _, svc := range services {
		if seenCompose[svc.compose] {
			t.Errorf("duplicate compose name %q", svc.compose)
		}
		if seenImage[svc.image] {
			t.Errorf("duplicate image name %q", svc.image)
		}
		seenCompose[svc.compose], seenImage[svc.image] = true, true
		if _, err := svc.binPath(); err != nil {
			t.Error(err)
		}
		if svc.pin == nil {
			t.Errorf("%s: no pin option", svc.compose)
		}
	}
	if len(operatorRun()) == 0 {
		t.Fatal("no operator-run services: nothing would ever be pinned")
	}

	// otherThan and the baseline set are two views of one table: for every
	// service under upgrade, the exclusion list is exactly the other rows.
	for _, up := range operatorRun() {
		got := otherThan(up)
		if len(got) != len(services)-1 {
			t.Errorf("otherThan(%s) = %v, want %d names", up.compose, got, len(services)-1)
		}
		for _, name := range got {
			if name == up.compose {
				t.Errorf("otherThan(%s) contains %s itself", up.compose, name)
			}
			if !seenCompose[name] {
				t.Errorf("otherThan(%s) names %q, which is not in the table", up.compose, name)
			}
		}
	}

	_, built, err := workspace.Detect()
	if err != nil {
		t.Logf("no active go.work (%v); skipping the workspace-selection check", err)
		return
	}
	want := make([]string, 0, len(services))
	for _, svc := range services {
		want = append(want, svc.compose)
	}
	sort.Strings(want)
	sort.Strings(built)
	if strings.Join(want, ",") != strings.Join(built, ",") {
		t.Errorf("the workspace would build %v from HEAD; the compat table covers %v — every built service must be asserted and every asserted service must be buildable", built, want)
	}
}
