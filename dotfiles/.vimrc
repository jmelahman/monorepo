""" Settings
set nocompatible      " Set compatibility to Vim only
set modelines=0       " Disable modelines
set encoding=utf-8
set spelllang=en      " English language for spellcheck
set spell             " Enable spellcheck by default
let mapleader = ","   " Set leader key

""" Searching
set ignorecase        " Case insensitive
set smartcase         " Use case if any caps are used
set hlsearch          " Highlight search
set incsearch         " Show match as search proceeds

""" Indenting
set tabstop=2         " Set tab width to 2
set shiftwidth=2      " Set indent to 2
set expandtab		      " Replace tabs with spaces
set autoindent        " Enable auto indent
set smartindent       " Enable smart indent

""" Formatting
set number            " Enable line numbers
set relativenumber    " Enable relative line numbers
set cursorline        " Highlight current lint
set textwidth=99      " Width of screen
set colorcolumn=+1    " Vertical ruler
syntax on             " Enable syntax highlighting
set nowrap            " Disable line wrapping
set scrolloff=3       " Minimum lines around cursor displayed
set listchars=tab:▸\ ,trail:•   " Configure Visualize whitespace
set list              " Enable whitespace visualtization
" Automatically remove trailing whitespace
autocmd BufWritePre * :%s/\s\+$//e

""" Highlighting
hi clear SpellBad
hi SpellBad cterm=underline

""" Plugins
" Typescript syntax highlighting
" git clone https://github.com/leafgarland/typescript-vim.git ~/.vim/pack/typescript/start/typescript-vim
