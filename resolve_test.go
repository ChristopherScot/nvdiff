package main

import (
	"encoding/json"
	"errors"
	"testing"
)

func mustStack(t *testing.T, raw string) stack {
	t.Helper()
	var s stack
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		t.Fatal(err)
	}
	return s
}

// exists is resolveRef over a fixed set of refs, sharing its preference order.
func exists(refs ...string) func(string) string {
	set := map[string]bool{}
	for _, r := range refs {
		set[r] = true
	}
	return func(branch string) string {
		for _, r := range refCandidates(branch) {
			if set[r] {
				return r
			}
		}
		return ""
	}
}

func TestPick(t *testing.T) {
	const (
		bottom    = `{"trunk":"main","currentBranch":"a","branches":[{"name":"a"},{"name":"b"}]}`
		middle    = `{"trunk":"main","currentBranch":"b","branches":[{"name":"a"},{"name":"b"}]}`
		allMerged = `{"trunk":"main","currentBranch":"c","branches":[{"name":"a","isMerged":true},{"name":"b","isMerged":true},{"name":"c"}]}`
		skipOne   = `{"trunk":"main","currentBranch":"c","branches":[{"name":"a"},{"name":"b","isMerged":true},{"name":"c"}]}`
		notIn     = `{"trunk":"main","currentBranch":"z","branches":[{"name":"a"}]}`
	)
	noPR := errors.New(`gh pr: no pull requests found for branch "x"`)

	for _, tc := range []struct {
		name    string
		f       facts
		refs    []string
		want    string
		wantWhy string
	}{
		{
			name:    "bottom layer uses trunk",
			f:       facts{stack: mustStack(t, bottom), prErr: noPR},
			refs:    []string{"origin/main"},
			want:    "origin/main",
			wantWhy: "stack (bottom layer): origin/main",
		},
		{
			name:    "open layer below",
			f:       facts{stack: mustStack(t, middle), prErr: noPR},
			refs:    []string{"origin/a", "origin/main"},
			want:    "origin/a",
			wantWhy: "stack (layer below): origin/a",
		},
		{
			// The bug that started this: every layer below merged and deleted.
			name:    "all below merged falls to trunk",
			f:       facts{stack: mustStack(t, allMerged), prErr: noPR},
			refs:    []string{"origin/main"},
			want:    "origin/main",
			wantWhy: "stack (layers below merged): origin/main",
		},
		{
			name:    "skips merged to nearest open",
			f:       facts{stack: mustStack(t, skipOne), prErr: noPR},
			refs:    []string{"origin/a", "origin/main"},
			want:    "origin/a",
			wantWhy: "stack (layer below): origin/a",
		},
		{
			name:    "local branch when origin has none",
			f:       facts{stack: mustStack(t, middle), prErr: noPR},
			refs:    []string{"a", "origin/main"},
			want:    "a",
			wantWhy: "stack (layer below): a",
		},
		{
			name:    "unmerged layer deleted falls to pr base",
			f:       facts{stack: mustStack(t, middle), pr: prInfo{BaseRefName: "main"}},
			refs:    []string{"origin/main"},
			want:    "origin/main",
			wantWhy: "stack (layer below): a not found; pr: origin/main",
		},
		{
			name:    "not in a stack uses pr base",
			f:       facts{stack: mustStack(t, notIn), pr: prInfo{BaseRefName: "dev"}},
			refs:    []string{"origin/dev", "origin/main"},
			want:    "origin/dev",
			wantWhy: "stack: z is not in the stack; pr: origin/dev",
		},
		{
			name:    "gh failing everywhere falls to main",
			f:       facts{stackErr: errors.New("gh stack: not logged in"), prErr: errors.New("gh pr: not logged in")},
			refs:    []string{"origin/main"},
			want:    "origin/main",
			wantWhy: "stack: gh stack: not logged in; pr: gh pr: not logged in; default: origin/main",
		},
		{
			name:    "nothing resolves",
			f:       facts{stackErr: errors.New("gh stack: no stack"), prErr: noPR},
			want:    "",
			wantWhy: `stack: gh stack: no stack; pr: gh pr: no pull requests found for branch "x"; default: main not found`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := pick(tc.f, exists(tc.refs...))
			if c.ref != tc.want || c.reason() != tc.wantWhy {
				t.Errorf("pick = (%q, %q)\n          want (%q, %q)", c.ref, c.reason(), tc.want, tc.wantWhy)
			}
		})
	}
}
