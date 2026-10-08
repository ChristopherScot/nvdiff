package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

type stack struct {
	Trunk         string `json:"trunk"`
	CurrentBranch string `json:"currentBranch"`
	Branches      []struct {
		Name     string `json:"name"`
		IsMerged bool   `json:"isMerged"`
	} `json:"branches"`
}

type prInfo struct {
	Number      int    `json:"number"`
	BaseRefName string `json:"baseRefName"`
}

// facts is everything the base choice depends on, fetched up front so pick
// stays pure.
type facts struct {
	stack    stack
	stackErr error
	pr       prInfo
	prErr    error
}

type choice struct {
	ref   string
	trail []string // why each earlier candidate was passed over, then why ref won
}

func (c choice) reason() string { return strings.Join(c.trail, "; ") }

// pick returns the first candidate that resolves: the nearest unmerged stack
// layer below (merged layers are already in trunk), then the PR's base, then
// main. resolve maps a branch to a ref that exists, or "" when none does.
func pick(f facts, resolve func(string) string) choice {
	var c choice
	try := func(source, branch string) bool {
		ref := resolve(branch)
		if ref == "" {
			c.trail = append(c.trail, fmt.Sprintf("%s: %s not found", source, branch))
			return false
		}
		c.ref = ref
		c.trail = append(c.trail, fmt.Sprintf("%s: %s", source, ref))
		return true
	}

	if f.stackErr != nil {
		c.trail = append(c.trail, "stack: "+f.stackErr.Error())
	} else if branch, why := stackBase(f.stack); branch == "" {
		c.trail = append(c.trail, "stack: "+why)
	} else if try("stack ("+why+")", branch) {
		return c
	}

	if f.prErr != nil {
		c.trail = append(c.trail, "pr: "+f.prErr.Error())
	} else if try("pr", f.pr.BaseRefName) {
		return c
	}

	try("default", "main")
	return c
}

func stackBase(s stack) (branch, why string) {
	for i, b := range s.Branches {
		if b.Name != s.CurrentBranch {
			continue
		}
		for j := i - 1; j >= 0; j-- {
			if !s.Branches[j].IsMerged {
				return s.Branches[j].Name, "layer below"
			}
		}
		if i == 0 {
			return s.Trunk, "bottom layer"
		}
		return s.Trunk, "layers below merged"
	}
	return "", s.CurrentBranch + " is not in the stack"
}

// runRaw returns a command's stdout untouched, for -z output where leading
// and trailing spaces belong to file names. On failure the error carries the
// command's own stderr, which is the useful part ("no pull requests found").
func runRaw(name string, args ...string) (string, error) {
	out, err := exec.Command(name, args...).Output()
	if err != nil {
		cmd := name
		if len(args) > 0 {
			cmd += " " + args[0]
		}
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			if msg := firstLine(string(exitErr.Stderr)); msg != "" {
				return "", fmt.Errorf("%s: %s", cmd, msg)
			}
		}
		return "", fmt.Errorf("%s: %w", cmd, err)
	}
	return string(out), nil
}

func run(name string, args ...string) (string, error) {
	out, err := runRaw(name, args...)
	return strings.TrimSpace(out), err
}

func firstLine(s string) string {
	s, _, _ = strings.Cut(strings.TrimSpace(s), "\n")
	return s
}

func gatherFacts(withStack bool) facts {
	var f facts
	if withStack {
		if out, err := run("gh", "stack", "view", "--json"); err != nil {
			f.stackErr = err
		} else if err := json.Unmarshal([]byte(out), &f.stack); err != nil {
			f.stackErr = fmt.Errorf("parsing gh stack view: %w", err)
		}
	}
	if out, err := run("gh", "pr", "view", "--json", "number,baseRefName"); err != nil {
		f.prErr = err
	} else if err := json.Unmarshal([]byte(out), &f.pr); err != nil {
		f.prErr = fmt.Errorf("parsing gh pr view: %w", err)
	}
	return f
}

// refCandidates prefers origin/: a local branch not pulled since a rebase is
// behind, and everything it is missing would show up as your own change.
func refCandidates(branch string) []string {
	return []string{"origin/" + branch, branch}
}

func isCommit(ref string) bool {
	_, err := run("git", "rev-parse", "--verify", "--quiet", ref+"^{commit}")
	return err == nil
}

func resolveRef(branch string) string {
	for _, ref := range refCandidates(branch) {
		if isCommit(ref) {
			return ref
		}
	}
	return ""
}

// diffBase is where the diff starts: the merge-base with ref, not ref's tip.
// Against the tip, every commit that landed since branching reads as yours.
type diffBase struct {
	ref, mergeBase, reason string
}

// resolveBase also returns the branch's PR number (0 if none), since the
// lookup that finds the PR's base finds it too.
func resolveBase(override string) (diffBase, int, error) {
	var b diffBase
	f := gatherFacts(override == "")

	if override != "" {
		if !isCommit(override) {
			return b, f.pr.Number, fmt.Errorf("--base %s: not a commit", override)
		}
		b.ref, b.reason = override, "--base"
	} else {
		c := pick(f, resolveRef)
		if c.ref == "" {
			return b, f.pr.Number, fmt.Errorf("no base found (%s)", c.reason())
		}
		b.ref, b.reason = c.ref, c.reason()
	}

	mb, err := run("git", "merge-base", "HEAD", b.ref)
	if err != nil {
		return b, f.pr.Number, err
	}
	b.mergeBase = mb
	return b, f.pr.Number, nil
}

func (b diffBase) String() string {
	short := b.mergeBase
	if len(short) > 10 {
		short = short[:10]
	}
	return fmt.Sprintf("nvdiff: base %s @ %s (%s)", b.ref, short, b.reason)
}
