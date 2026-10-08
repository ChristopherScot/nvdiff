# nvdiff

Owned by me-myself-and-i.

Open the files your branch changed in nvim, as a quickfix list, with each
entry on the file's first changed line.

## Usage

```sh
nvdiff                 # open nvim on the changed files
nvdiff files           # print them instead, one per line
nvdiff --base <ref>    # diff from <ref> instead of working one out
```

"Changed" means the working tree against the merge-base, so uncommitted
edits count, and so does every untracked file that isn't gitignored.

The base is the first of these that exists, preferring `origin/<branch>`
over a local branch that may be behind:

1. the nearest stack layer below that hasn't merged (`gh stack`); merged
   layers are already in trunk, so a stack whose lower layers have all
   landed uses trunk
2. the PR's base branch
3. `main`

Every run prints which base it used and why, to stderr:

```
nvdiff: base origin/main @ 58240b638e (stack (layers below merged): origin/main)
```

`nvdiff` opens nvim at the repo root. `nvdiff files` prints paths relative
to wherever you run it, so `nvim $(nvdiff files)` works from a subdirectory.

## nvim integration

The quickfix setup is built in, so nothing is needed in your nvim config.
nvdiff also exports these for config that wants them:

| variable | value |
|---|---|
| `GITSIGNS_BASE` | the merge-base commit. Pass it as gitsigns' `base` option so the gutter marks this branch's changes, not just uncommitted ones |
| `NVDIFF_PR` | the PR number. Unset when the branch has no PR |

```lua
require("gitsigns").setup({ base = vim.env.GITSIGNS_BASE })
```

## Install

```sh
curl -sL https://github.com/ChristopherScot/nvdiff/releases/latest/download/nvdiff_darwin_arm64.tar.gz | tar xz
mv nvdiff ~/bin/
xattr -d com.apple.quarantine ~/bin/nvdiff   # macOS only
```

After that, `nvdiff update`. Needs `git`, `gh` and `nvim`. Without the
`gh stack` extension, step 1 above is skipped.

## Build

```sh
make build            # compile
make run ARGS='...'   # run without installing
```

Releases are built by CI and published to GitHub Releases;
`nvdiff update` pulls the newest one.

## Testing

```sh
make test             # go vet and go test -race. What CI runs.
```

Add a service's own setup to `Makefile.local`. It is not generated, so
it survives a regenerate.

## Deploy

Not deployed — this is a binary people install, not a service. CI
attaches one per platform to a GitHub Release when the version in
`VERSION` changes.
