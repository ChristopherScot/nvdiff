package main

import (
	"reflect"
	"testing"
)

func TestNvimEnv(t *testing.T) {
	// What a :terminal inside an earlier nvdiff session inherits.
	inherited := []string{
		"HOME=/h",
		"GITSIGNS_BASE=old",
		"NVDIFF_PR=12",
		"NVDIFF_FILES=[]",
		"NVDIFF_PROMPT=keep", // shares a prefix, not a name
	}

	got := nvimEnv(inherited, "new", 0, []byte(`[{"path":"a","line":1}]`))
	want := []string{
		"HOME=/h",
		"NVDIFF_PROMPT=keep",
		`NVDIFF_FILES=[{"path":"a","line":1}]`,
		"GITSIGNS_BASE=new",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("no PR:\n got %q\nwant %q", got, want)
	}

	got = nvimEnv(inherited, "new", 34, []byte(`[]`))
	if got[len(got)-1] != "NVDIFF_PR=34" {
		t.Errorf("with PR: got %q, want NVDIFF_PR=34 last", got)
	}
}
