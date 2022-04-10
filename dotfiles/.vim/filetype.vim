if exists('did_load_filetypes')
    finish
endif

augroup filetypedetect
autocmd BufNewFile,BufRead *.rc        if getline(1) =~# '^#!/usr/bin/bash\>' | setf bash | endif
augroup END
