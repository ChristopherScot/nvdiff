// nvdiff
package main

// THIS WHOLE REPO IS YOURS. Nothing rewrites these files after `init`.
// Two couplings survive that:
//
//   - VERSION drives releases: CI publishes only when its first line
//     changes on main, tagging what it says.
//   - update.go downloads `nvdiff_<os>_<arch>.tar.gz` from the
//     latest GitHub release; CI builds assets with exactly that name.
//     Rename either side and self-update fails.

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

// Version is set at build time via ldflags; "dev" refuses to self-update.
var Version = "dev"

func rootCmd() *cobra.Command {
	var base string
	root := &cobra.Command{
		Use:   "nvdiff",
		Short: "open the current PR's changed files in nvim",
		Long: "Open this branch's changed files in nvim as a quickfix list, diffed from\n" +
			"its merge-base with the nearest unmerged stack layer below, else the PR's\n" +
			"base, else main.",
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true, // main prints them
		RunE: func(_ *cobra.Command, _ []string) error {
			r, err := prepare(base)
			if err != nil {
				return err
			}
			return openNvim(r.base, r.pr, r.files)
		},
	}
	root.PersistentFlags().StringVar(&base, "base", "", "diff from this ref instead of resolving one")
	root.AddCommand(filesCmd(&base), updateCmd(), versionCmd())

	// Cobra builds `completion` during Execute, so force it to exist now
	// and hang `install` off it.
	root.InitDefaultCompletionCmd()
	for _, c := range root.Commands() {
		if c.Name() == "completion" {
			c.AddCommand(completionInstallCmd("nvdiff"))
		}
	}
	return root
}

func filesCmd(base *string) *cobra.Command {
	return &cobra.Command{
		Use:   "files",
		Short: "print the changed files, one per line, relative to where you are",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			// Resolved, because git reports the root with symlinks resolved
			// (/private/tmp, not /tmp) and Rel needs both sides alike.
			cwd, err := os.Getwd()
			if err == nil {
				cwd, err = filepath.EvalSymlinks(cwd)
			}
			if err != nil {
				return err
			}
			r, err := prepare(*base)
			if err != nil {
				return err
			}
			for _, f := range r.files {
				p, err := filepath.Rel(cwd, filepath.Join(r.root, f.Path))
				if err != nil {
					p = f.Path
				}
				fmt.Println(p)
			}
			return nil
		},
	}
}

type review struct {
	root  string
	base  diffBase
	pr    int // 0 when the branch has no PR
	files []file
}

// prepare moves to the repo root, which git's paths are relative to, then
// resolves the base and lists files. The base goes to stderr so stdout stays
// clean for `nvim $(nvdiff files)`.
func prepare(override string) (review, error) {
	var r review
	root, err := run("git", "rev-parse", "--show-toplevel")
	if err != nil {
		return r, err
	}
	if err := os.Chdir(root); err != nil {
		return r, err
	}
	r.root = root
	if r.base, r.pr, err = resolveBase(override); err != nil {
		return r, err
	}
	fmt.Fprintln(os.Stderr, r.base)
	r.files, err = changedFiles(r.base.mergeBase)
	return r, err
}

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "print the version",
		Args:  cobra.NoArgs,
		Run: func(_ *cobra.Command, _ []string) {
			fmt.Println(Version)
		},
	}
}

func main() {
	if err := rootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

// makeCompleter returns ShellCompDirectiveError rather than nothing so the
// shell does not fall back to filenames, which would look like a bug.
func makeCompleter(list func() ([]string, error)) func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return func(_ *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
		if len(args) != 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		names, err := list()
		if err != nil {
			return nil, cobra.ShellCompDirectiveError
		}
		return names, cobra.ShellCompDirectiveNoFileComp
	}
}
