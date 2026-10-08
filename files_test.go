package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
)

func TestFirstHunks(t *testing.T) {
	diff := "diff --git a/edited.go b/edited.go\n" +
		"--- a/edited.go\n" +
		"+++ b/edited.go\n" +
		"@@ -10,0 +11,2 @@ func x() {\n" +
		"+a\n" +
		"@@ -40 +42 @@\n" +
		"+d\n" +
		"diff --git a/new.go b/new.go\n" +
		"--- /dev/null\n" +
		"+++ b/new.go\n" +
		"@@ -0,0 +1,3 @@\n" +
		"+--- a/fake.go\n" +
		"+++ b/fake.go\n" +
		"diff --git a/gone.go b/gone.go\n" +
		"--- a/gone.go\n" +
		"+++ /dev/null\n" +
		"@@ -1,2 +0,0 @@\n" +
		"---- a/also-fake.go\n" +
		"diff --git a/sp ace.go b/sp ace.go\n" +
		"--- a/sp ace.go\t\n" +
		"+++ b/sp ace.go\t\n" +
		"@@ -5 +7 @@\n" +
		"diff --git a/café.go b/café.go\n" +
		"--- a/café.go\n" +
		"+++ b/café.go\n" +
		"@@ -2 +3 @@\n" +
		"diff --git a/img.png b/img.png\n" +
		"Binary files a/img.png and b/img.png differ\n"

	got := firstHunks(diff)
	want := map[string]int{
		"edited.go": 11, // first hunk only, not 42
		"new.go":    1,
		"gone.go":   0, // pure deletion
		"sp ace.go": 7, // git appends a tab after a name with a space
		"café.go":   3,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("firstHunks = %v\n         want %v", got, want)
	}
}

func TestMergeLists(t *testing.T) {
	got := mergeLists(
		[]string{"a.go", "gone.go", ""},
		[]string{"new.go", "a.go"},
		map[string]int{"a.go": 4, "gone.go": 0},
	)
	want := []file{{"a.go", 4}, {"gone.go", 1}, {"new.go", 1}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("mergeLists = %v, want %v", got, want)
	}

	data, _ := json.Marshal(mergeLists(nil, nil, nil))
	if string(data) != "[]" {
		t.Errorf("no files marshals as %s; qf.lua needs []", data)
	}
}

// TestChangedFiles runs against a real repo, with a global git config that
// sets the options which change diff headers.
func TestChangedFiles(t *testing.T) {
	dir := t.TempDir()
	gitconfig := filepath.Join(t.TempDir(), "gitconfig")
	writeFile(t, gitconfig, "[diff]\n\tnoprefix = true\n\tmnemonicPrefix = true\n"+
		"[user]\n\tname = t\n\temail = t@example.com\n")
	t.Setenv("GIT_CONFIG_GLOBAL", gitconfig)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Chdir(dir)

	git := func(args ...string) {
		t.Helper()
		if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	git("init", "-q")
	writeFile(t, "edited.go", "1\n2\n3\n4\n5\n")
	writeFile(t, "gone.go", "x\n")
	writeFile(t, "café.go", "1\n2\n")
	writeFile(t, ".gitignore", "ignored.txt\n")
	git("add", ".")
	git("commit", "-qm", "base")

	writeFile(t, "edited.go", "1\n2\n3\nfour\n5\n")
	writeFile(t, "café.go", "1\ntwo\n")
	if err := os.Remove("gone.go"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir("new dir", 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, "new dir/x y.go", "new\n")
	writeFile(t, "ignored.txt", "noise\n")

	got, err := changedFiles("HEAD")
	if err != nil {
		t.Fatal(err)
	}
	want := []file{{"café.go", 2}, {"edited.go", 4}, {"gone.go", 1}, {"new dir/x y.go", 1}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("changedFiles = %v\n          want %v", got, want)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
