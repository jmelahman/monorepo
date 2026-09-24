-- Personal configurations
vim.opt.compatible = false      -- Set compatibility to Vim only
vim.opt.modelines = 0           -- Disable modelines
vim.opt.encoding = 'utf-8'
vim.opt.spelllang = 'en'        -- English language for spellcheck
vim.opt.spell = true            -- Enable spellcheck by default
vim.opt.signcolumn = "no"       -- Disable vertical gutter
vim.opt.clipboard = "unnamedplus" -- Enable the clipboard

-- Searching
vim.opt.ignorecase = true       -- Case insensitive
vim.opt.smartcase = true        -- Use case if any caps are used
vim.opt.hlsearch = true         -- Highlight search
vim.opt.incsearch = true        -- Show match as search proceeds

-- Indenting
vim.opt.tabstop = 2             -- Set tab width to 2
vim.opt.shiftwidth = 2          -- Set indent to 2
vim.opt.expandtab = true        -- Replace tabs with spaces
vim.opt.autoindent = true       -- Enable auto indent
vim.opt.smartindent = true      -- Enable smart indent

-- Folding
vim.opt.foldmethod = "indent"
vim.opt.foldlevel = 99          -- Open all folds by default

-- Formatting
vim.opt.number = true           -- Enable line numbers
vim.opt.relativenumber = true   -- Enable relative line numbers
vim.opt.cursorline = true       -- Highlight current line
vim.opt.textwidth = 99          -- Width of screen
vim.opt.colorcolumn = '+1'      -- Vertical ruler
vim.cmd('syntax on')            -- Enable syntax highlighting
vim.opt.wrap = false            -- Disable line wrapping
vim.opt.scrolloff = 5           -- Minimum lines around cursor displayed
vim.opt.listchars = { tab = '▸ ', trail = '•' }  -- Configure Visualize whitespace
vim.opt.list = true             -- Enable whitespace visualization

-- Menus
vim.opt.pumheight = 4           -- Limit menus to 4 items

-- Highlighting
vim.cmd('hi clear SpellBad')
vim.cmd('hi SpellBad cterm=underline')

-- Colorscheme configuration
vim.cmd [[ colorscheme jvim ]]

-- Syntax highlighting
vim.cmd [[ au BufRead,BufNewFile PKGBUILD set filetype=sh ]]

-- Key mappings
vim.g.mapleader = " "
vim.g.maplocalleader = "\\"

-- Automatically remove trailing whitespace
vim.api.nvim_create_autocmd('BufWritePre', {
  pattern = '*',
  command = [[%s/\s\+$//e]]
})
