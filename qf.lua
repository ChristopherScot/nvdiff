-- Run by nvdiff via -c. Data comes from env, never from the command string.
--
-- Env contract (set by nvdiff, see nvim.go):
--   NVDIFF_FILES   JSON array of {path, line}; always set, possibly []
--   GITSIGNS_BASE  merge-base commit, for a gitsigns `base` option
--   NVDIFF_PR      PR number; unset when the branch has no PR
local raw = vim.env.NVDIFF_FILES
-- Read once; :terminal and LSP children have no use for it.
vim.env.NVDIFF_FILES = nil

local ok, files = pcall(vim.json.decode, raw or "")
if not ok or type(files) ~= "table" then
    vim.notify("nvdiff: could not read NVDIFF_FILES", vim.log.levels.ERROR)
    return
end
if #files == 0 then
    vim.notify("nvdiff: no changed files vs " .. (vim.env.GITSIGNS_BASE or "?"), vim.log.levels.WARN)
    return
end

-- filename, not bufnr: auto-session wipes and recreates buffers during
-- restore, so a captured number dangles (E92 on the first jump).
local items = {}
for _, f in ipairs(files) do
    table.insert(items, {
        filename = f.path,
        lnum = f.line,
        col = 1,
        -- :copen and trouble.nvim render this; without it they show "||".
        text = vim.fn.fnamemodify(f.path, ":t"),
    })
end

-- Deferred past startup so a session restore cannot undo it. cfirst edits the
-- entry, which runs filetype detection; an unloaded buffer gets no treesitter
-- or LSP.
vim.schedule(function()
    vim.fn.setqflist(items, "r")
    vim.cmd("silent! cfirst")
end)
