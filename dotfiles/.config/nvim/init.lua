require("core.bootstrap")
require("core.options")

-- Must be before the lazyvim setup.
vim.g.coq_settings = {
  auto_start = "shut-up",
  display = {
    icons = { mode = "none" },
    pum = {
      y_max_len = 4,     -- cap height to 4; hack around coq's _update_pumheight()
    },
  },
}

-- Setup lazy.nvim
require("lazy").setup({
  spec = {
    {
      "neovim/nvim-lspconfig",
      lazy = false,
      dependencies = {
        { "ms-jpq/coq_nvim", branch = "coq" },
        { "ms-jpq/coq.artifacts", branch = "artifacts" },
        { 'ms-jpq/coq.thirdparty', branch = "3p" }
      },
      config = function()
        local coq = require("coq")

        -- Server configurations shared by both old and new APIs.
        local servers = {
          pyright = {},
          ruff = {
            cmd = { "uv", "tool", "run", "ruff", "server" },
          },
          ts_ls = {
            on_attach = function(client)
              client.server_capabilities.documentFormattingProvider = false
              client.server_capabilities.documentRangeFormattingProvider = false
            end,
          },
          gopls = {
            on_attach = function(client)
              if client.server_capabilities.documentFormattingProvider then
                vim.api.nvim_create_autocmd("BufWritePre", {
                  group = vim.api.nvim_create_augroup("GoFormat", { clear = true }),
                  pattern = '*.go',
                  callback = function() vim.lsp.buf.format() end
                })
              end
            end,
          },
          rust_analyzer = {
            capabilities = vim.lsp.protocol.make_client_capabilities(),
            on_attach = function(client)
              if client.server_capabilities.documentFormattingProvider then
                vim.api.nvim_create_autocmd("BufWritePre", {
                  group = vim.api.nvim_create_augroup("RustFmt", { clear = true }),
                  pattern = '*.rs',
                  callback = function() vim.lsp.buf.format() end,
                })
              end
            end,
          },
          yamlls = {
            settings = {
              yaml = {
                schemas = {
                  ["https://json.schemastore.org/github-workflow.json"] = "/.github/workflows/*",
                  ["https://json.schemastore.org/prettierrc.json"] = "/.prettierrc.{yml,yaml}",
                  kubernetes = "/*.yaml",
                },
                format = { enable = true },
                validate = true,
              },
            },
          },
        }

        if vim.lsp.config then
          -- Neovim 0.11+ native LSP config API
          for name, cfg in pairs(servers) do
            vim.lsp.config(name, cfg)
            vim.lsp.enable(name)
          end
        else
          -- Neovim 0.10 and earlier — use nvim-lspconfig
          local lspconfig = require("lspconfig")
          for name, cfg in pairs(servers) do
            lspconfig[name].setup(coq.lsp_ensure_capabilities(cfg))
          end
        end
      end,
    },
    {
      "ray-x/go.nvim",
      dependencies = {
        "ray-x/guihua.lua",
        "neovim/nvim-lspconfig",
        "nvim-treesitter/nvim-treesitter",
      },
      config = function()
        require("go").setup()
      end,
      event = {"CmdlineEnter"},
      ft = {"go", 'gomod'},
      build = ':lua require("go.install").update_all_sync()'
    },
    {
      "hashivim/vim-terraform",
      event = "BufRead *.tf,*.tfvars,*.tfstate",
      lazy = false,
      config = function()
        -- Optional: Autoformat terraform files
        vim.g.terraform_fmt_on_save = 1
        vim.g.terraform_align = 1

        -- Enable terraform filetype detection
        vim.cmd([[
          autocmd BufRead,BufNewFile *.tf set filetype=terraform
          autocmd BufRead,BufNewFile *.tfstate set filetype=json
          autocmd BufRead,BufNewFile *.tfvars set filetype=terraform
        ]])
      end
    },
    {
      "hat0uma/csvview.nvim",
      ft = { "csv", "tsv" },
      config = function()
        require("csvview").setup()
      end,
    },
    {
      "cgrindel/vim-bazelrc",
      ft = "bazelrc",
      event = "BufRead *.bazelrc",
    },
    {
      "kawre/leetcode.nvim",
      event = "CmdlineEnter",
      build = ":TSUpdate html",
      dependencies = {
        "MunifTanjim/nui.nvim",
        "nvim-lua/plenary.nvim",
        "nvim-telescope/telescope.nvim",
        "nvim-treesitter/nvim-treesitter",
      },
      opts = {
        lang = "python3",
        description = {
          position = "top",
        },
      },
    },
    {
      "nvimtools/none-ls.nvim",
      config = function()
        local null_ls = require("null-ls")
        null_ls.setup({
          sources = {
            null_ls.builtins.diagnostics.golangci_lint,
            null_ls.builtins.formatting.prettier.with({
              filetypes = {
                "javascript", "javascriptreact", "typescript", "typescriptreact",
                "json", "yaml", "html", "css", "scss", "markdown",
              },
            }),
          },
          on_attach = function(client, bufnr)
            if client.supports_method("textDocument/formatting") then
              vim.api.nvim_create_autocmd("BufWritePre", {
                buffer = bufnr,
                callback = function()
                  vim.lsp.buf.format({ bufnr = bufnr, async = false, filter = function(client) return client.name == "null-ls" end })
                end,
              })
            end
          end,
        })
      end,
      dependencies = {
        "nvim-lua/plenary.nvim",
      },
    },
  },
  -- automatically check for plugin updates
  checker = { enabled = true , notify = false },
  performance = {
    cache = {
      enabled = true
    },
    reset_packpath = true,
    rtp = {
      reset = true,
      paths = {}
    }
  }
})

local status_ok, treesitter_configs = pcall(require, 'nvim-treesitter.configs')
if status_ok then
  treesitter_configs.setup {
    ensure_installed = {"python", "typescript", "go", "rust", "yaml"},
    highlight = {
      enable = true,
      additional_vim_regex_highlighting = false,
    },
    indent = {
      enable = true
    },
    incremental_selection = {
      enable = true,
      keymaps = {
        init_selection = "gnn",
        node_incremental = "grn",
        scope_incremental = "grc",
        node_decremental = "grm",
      },
    },
  }
end

vim.keymap.set('n', '<leader>gi', '<cmd>GoImports<CR>', { desc = "Run GoImports" })
vim.keymap.set('n', '<leader>ca', '<cmd>lua vim.lsp.buf.code_action()<CR>', { desc = "Code Actions" })
vim.keymap.set("n", "gd", vim.lsp.buf.definition, { noremap = true, silent = true })
vim.keymap.set("n", "gD", vim.lsp.buf.declaration, { noremap = true, silent = true })
vim.keymap.set("n", "gr", vim.lsp.buf.references, { noremap = true, silent = true })
vim.keymap.set("n", "K", vim.lsp.buf.hover, { noremap = true, silent = true })

-- Show diagnostics automatically when cursor holds
vim.api.nvim_create_autocmd("CursorHold", {
  callback = function()
    vim.diagnostic.open_float(nil, { focus = false, scope = "cursor" })
  end,
})

-- Set how long to wait before showing diagnostics (in milliseconds)
vim.opt.updatetime = 300
