package main

import (
	"strconv"
	"strings"
)

// file is the contract with qf.lua: git-level facts only. Turning these into
// quickfix entries is nvim's business.
type file struct {
	Path string `json:"path"`
	Line int    `json:"line"`
}

// changedFiles lists what the working tree changed since base, plus every
// untracked non-ignored file: a brand-new file is often the one most worth
// reviewing, and git diff never reports it.
func changedFiles(base string) ([]file, error) {
	names, err := runRaw("git", "diff", "--name-only", "-z", base)
	if err != nil {
		return nil, err
	}
	untracked, err := runRaw("git", "ls-files", "--others", "--exclude-standard", "-z")
	if err != nil {
		return nil, err
	}
	// firstHunks parses headers, so pin their format against user config:
	// unquoted names, a/ b/ prefixes, and no external diff tool.
	hunks, err := runRaw("git", "-c", "core.quotePath=false", "diff", "-U0",
		"--no-color", "--no-ext-diff", "--src-prefix=a/", "--dst-prefix=b/", base)
	if err != nil {
		return nil, err
	}
	return mergeLists(splitNUL(names), splitNUL(untracked), firstHunks(hunks)), nil
}

func splitNUL(s string) []string {
	return strings.Split(strings.TrimSuffix(s, "\x00"), "\x00")
}

// mergeLists returns a non-nil slice, so no changes marshal as [] rather than
// null, which qf.lua would reject.
func mergeLists(tracked, untracked []string, first map[string]int) []file {
	files := []file{}
	seen := map[string]bool{}
	for _, list := range [][]string{tracked, untracked} {
		for _, p := range list {
			if p == "" || seen[p] {
				continue
			}
			seen[p] = true
			// 0 is a pure deletion; there is no line 0 to sit on.
			files = append(files, file{Path: p, Line: max(first[p], 1)})
		}
	}
	return files
}

// firstHunks maps each path in a -U0 diff to the first line its first hunk
// touches in the new file. It expects the format changedFiles pins.
func firstHunks(diff string) map[string]int {
	first := map[string]int{}
	var path string
	// ---/+++ only mean a file header before the first @@; inside a hunk they
	// are a removed "--" line or an added "++" line.
	header := false
	for line := range strings.Lines(diff) {
		line = strings.TrimSuffix(line, "\n")
		switch {
		case strings.HasPrefix(line, "diff --git "):
			header, path = true, ""
		case header && strings.HasPrefix(line, "+++ b/"):
			path = headerPath(line, "+++ b/")
		case header && strings.HasPrefix(line, "--- a/"):
			// A deleted file has no +++ b/ side; key it by its old path.
			path = headerPath(line, "--- a/")
		case strings.HasPrefix(line, "@@ ") && path != "":
			header = false
			if _, ok := first[path]; ok {
				continue
			}
			_, after, ok := strings.Cut(line, " +")
			if !ok {
				continue
			}
			num, _, _ := strings.Cut(after, ",")
			num, _, _ = strings.Cut(num, " ")
			if n, err := strconv.Atoi(num); err == nil {
				first[path] = n
			}
		}
	}
	return first
}

// headerPath strips git's trailing tab, which it adds after a name containing
// a space.
func headerPath(line, prefix string) string {
	return strings.TrimSuffix(strings.TrimPrefix(line, prefix), "\t")
}
