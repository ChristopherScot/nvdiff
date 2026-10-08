package main

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

//go:embed qf.lua
var qfLua string

// openNvim replaces this process with nvim. The env contract is documented
// in qf.lua, which is what reads it.
func openNvim(b diffBase, pr int, files []file) error {
	nvim, err := exec.LookPath("nvim")
	if err != nil {
		return err
	}
	data, err := json.Marshal(files)
	if err != nil {
		return err
	}
	err = syscall.Exec(nvim, []string{"nvim", "-c", "lua " + qfLua}, nvimEnv(os.Environ(), b.mergeBase, pr, data))
	if errors.Is(err, syscall.E2BIG) {
		return fmt.Errorf("%d changed files is too many to hand to nvim; if most are untracked, gitignore them", len(files))
	}
	return err
}

// nvimEnv drops any inherited copies first: syscall.Exec, unlike exec.Cmd,
// keeps duplicates, and getenv returns the first, so a nested nvdiff (run
// from :terminal) would otherwise see the outer session's values.
func nvimEnv(environ []string, mergeBase string, pr int, files []byte) []string {
	env := make([]string, 0, len(environ)+3)
	for _, kv := range environ {
		key, _, _ := strings.Cut(kv, "=")
		if key != "NVDIFF_FILES" && key != "GITSIGNS_BASE" && key != "NVDIFF_PR" {
			env = append(env, kv)
		}
	}
	env = append(env, "NVDIFF_FILES="+string(files), "GITSIGNS_BASE="+mergeBase)
	if pr != 0 {
		env = append(env, "NVDIFF_PR="+strconv.Itoa(pr))
	}
	return env
}
