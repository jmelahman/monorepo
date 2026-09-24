# Dotfiles

## Initialize

```shell
git clone --bare git@github.com:jmelahman/dotfiles.git $HOME/.dotfiles
alias dotfiles='git --git-dir=$HOME/.dotfiles --work-tree=$HOME'
dotfiles config --local status.showUntrackedFiles no
GIT_LFS_SKIP_SMUDGE=0 dotfiles checkout --force
```

## Installing `dot-sync`

`dot-sync` can be enabled to automatically sync (push and pull) changes every 10 minutes.

```shell
systemctl enable --now --user dot-sync.timer
```

## Enabling Git maintenance

The timers run `git maintenance run` on the repositories listed under `[maintenance]` in `.gitconfig`.

```shell
systemctl enable --now --user git-maintenance@{hourly,daily,weekly}.timer
```

The units are the ones `git maintenance start` writes. Skip that command here: it also appends the current repository to `~/.gitconfig` by absolute path, duplicating its `~/` entry.
