# Enable completion
autoload -Uz compinit add-zsh-hook
compinit
# add-zsh-hook chpwd auto_activate_venv

# History
HISTFILE=~/.zsh_history
HISTSIZE=10000
SAVEHIST=10000
setopt HIST_IGNORE_ALL_DUPS
setopt SHARE_HISTORY

# Tab completion tweaks
zstyle ':completion:*' menu select
zstyle ':completion:*' matcher-list 'm:{a-zA-Z}={A-Za-z}'
zstyle ':completion:*' group-name ''
zstyle ':completion:*:descriptions' format '%F{green}%d%f'
zstyle ':completion:*' list-colors ''

# Options
setopt autocd              # cd into directories without typing 'cd'
setopt correct             # auto-correct commands
setopt no_beep             # disable bell
setopt prompt_subst        # allow command substitution in prompt

bindkey -e                 # enable emacs bindings

# Ctrl+Left/Right word movement
bindkey "^[[1;5D" backward-word  # Ctrl+Left
bindkey "^[[1;5C" forward-word   # Ctrl+Right

# Alternate keycodes for some terminals
bindkey "^[OD" backward-word
bindkey "^[OC" forward-word

bindkey "\e[H" beginning-of-line # Home
bindkey "\e[F" end-of-line       # End

# Make Alt+Backspace behave like bash
bindkey '^[^?' backward-kill-word
WORDCHARS=''

# Make Shift+Tab go backwards for autocomplete
bindkey '^[[Z' reverse-menu-complete

fzf_git_files() {
  local file
  file=$(git diff --name-only | fzf --multi) || return
  LBUFFER+="$file "
  zle reset-prompt
}
zle -N fzf_git_files
bindkey '^D' fzf_git_files

# Prompt
ZSH_FIRST_PROMPT=1
autoload -Uz colors && colors

# Fast dirty check via git porcelain
parse_git_info() {
  local branch dirty marks
  command git rev-parse --is-inside-work-tree &>/dev/null || return

  branch=$(git symbolic-ref --short HEAD 2>/dev/null)
  [[ -z $branch ]] && return

  dirty=$(git status --porcelain 2>/dev/null)

  # Match only the two status columns of each line, so a path containing
  # "M " or "D " can't masquerade as a staged change.
  local line xy
  local -i mod=0 untracked=0 added=0 deleted=0
  for line in ${(f)dirty}; do
    xy=${line[1,2]}
    if [[ $xy == '??' ]]; then
      untracked=1
    else
      [[ $xy == *M* ]] && mod=1
      [[ $xy == *A* ]] && added=1
      [[ $xy == *D* ]] && deleted=1
    fi
    (( mod && untracked && added && deleted )) && break
  done

  marks=""
  (( mod ))       && marks+="!"
  (( untracked )) && marks+="?"
  (( added ))     && marks+="+"
  (( deleted ))   && marks+="x"
  [[ $(git rev-list --count --left-only @{u}...HEAD 2>/dev/null) -gt 0 ]] && marks+="*"

  echo " [$branch${marks:+ $marks}]"
}

# Get current terraform workspace
parse_terraform_workspace() {
  local workspace
  # Ignore directories that are not initialized with terraform.
  [[ -d ".terraform" ]] || return

  # Get the current workspace (suppress errors if terraform is not available or not in terraform dir)
  workspace=$(terraform workspace show 2>/dev/null)
  [[ -z $workspace ]] && return

  echo " (tf:$workspace)"
}

activate_default_venv() {
  [[ -f ~/code/onyx/.venv/bin/activate ]] && source ~/code/onyx/.venv/bin/activate
}

# Automatically activate a Python virtual environment if one exists
auto_activate_venv() {
    # Look for a virtual environment folder in the current directory
    if [[ -f ".venv/bin/activate" ]]; then
        # Only activate if not already active
        if [[ -z "$VIRTUAL_ENV" || "$VIRTUAL_ENV" != "$PWD/.venv" ]]; then
            source .venv/bin/activate
            # Refresh autocomplete to pick up any new binaries.
            compinit
        fi
    else
        # Deactivate if leaving a directory with a venv
        if [[ -n "$VIRTUAL_ENV" && "$VIRTUAL_ENV" == "$OLDPWD/.venv" ]]; then
            deactivate
            activate_default_venv
        fi
    fi
}
activate_default_venv

kube_context_info() {
  local ctx short cfg="${KUBECONFIG:-$HOME/.kube/config}"
  # Read the file directly; `kubectl config current-context` costs ~110ms of
  # binary startup on every prompt. A colon-separated KUBECONFIG has to be
  # merged by kubectl, so that case keeps the slow path.
  if [[ $cfg == *:* ]]; then
    ctx="$(kubectl config current-context 2>/dev/null)"
  else
    ctx="$(awk '/^current-context:/{print $2; exit}' "$cfg" 2>/dev/null)"
  fi
  [[ -z "$ctx" ]] && return
  short="${ctx##*/}"
  echo " ⎈ $short"
}

precmd() {
  local exit_code=$?
  local git_info host_info terraform_info
  git_info=$(parse_git_info)
  terraform_info=$(parse_terraform_workspace)
  kube_info=$(kube_context_info)
  host_info=""
  [[ -n $SSH_CONNECTION ]] && host_info=" (%{$fg[yellow]%}$(hostname)%{$reset_color%})"

  local color=$fg[green]
  (( exit_code != 0 )) && color=$fg[red]

  # Only prepend newline if this is not the first prompt
  local newline=""
  (( ZSH_FIRST_PROMPT == 0 )) && newline=$'\n'

  PROMPT="${newline}[%{$color%}$exit_code%{$reset_color%}] %{$fg[blue]%}%~%{$reset_color%}%{$fg[green]%}$git_info%{$reset_color%}%{$fg[cyan]%}$terraform_info%{$reset_color%}%{$fg[yellow]%}$kube_info%{$reset_color%}$host_info %D{%F %T}"
  PROMPT+=$'\n'"${PROMPT_CHAR:-$([[ $EUID -eq 0 ]] && echo '#' || echo '$')} "

  ZSH_FIRST_PROMPT=0
}

# Aliases
alias alert='notify-send --urgency=low -i "$([ $? = 0 ] && echo terminal || echo error)" "$(history|tail -n1|sed -e '\''s/^\s*[0-9]\+\s*//;s/[;&|]\s*alert$//'\'')"'
alias clear='clear && ZSH_FIRST_PROMPT=1'
alias gs='git status'
alias gl="git log --graph --pretty='%Cred%h%Creset -%C(yellow)%d%Creset %s %Cgreen(%cr) %C(bold blue)<%an>%Creset'"
alias gr='git reset --soft HEAD~1 && git commit --amend --no-edit'
alias gg='git log --graph --oneline --all --decorate'
alias gb='git for-each-ref --sort=-committerdate refs/heads/'
alias gt="git log --no-walk --tags --pretty='%Cred%h%Creset -%C(yellow)%d%Creset %s %Cgreen(%cr) %C(bold blue)<%an>%Creset' --decorate=full"
alias grep='grep --color=auto'
alias kc="kubectl"
alias nit='git commit -am "nit"'
alias kbdoff="sudo sys76-kb set -b 0"
alias less='less -R'
alias ls='ls --color=auto'
alias ll='ls -lh'
alias rg='rg --color=always'
alias power="upower -i /org/freedesktop/UPower/devices/battery_BAT0"
alias stop="pkill -STOP"
alias resume="pkill -CONT"
alias hostname="cat /etc/hostname"
alias update="sudo nixos-rebuild switch"
alias vim="nvim"
alias kitty="kitty --session ~/.config/kitty/startup-session.conf"
alias zwift="CONTAINER_TOOL='podman' zwift"
# TODO: Remove python3.12 once https://github.com/Aider-AI/aider/issues/3660 is resolved.
alias aider="uvx --python=3.12 --from=aider-chat aider"
alias ruff="uvx ruff"
alias pkillgrep='function _pg() { ps aux | grep "$1" | grep -v grep | awk "{print \$2}" | xargs -r kill; }; _pg'
alias fix-monitors="swaymsg output DP-1 position 0 0 && swaymsg output eDP-1 position 2560 0"
alias enable="swaymsg output eDP-1 enable"
alias disable="swaymsg output eDP-1 disable"
alias snip="slurp | grim -g -"
alias snap="sleep 2 && swaymsg -t get_tree | jq -r '.. | select(.focused?) | .rect | \"\(.x),\(.y) \(.width)x\(.height)\"' | grim -g -"
alias chat="ollama run mistral-small3.2:latest"
alias coder="ollama run qwen3-coder:latest"
alias weak="ollama run jqwen3:0.6b"
alias lights='smart-lights'
alias awslogin="aws sso login"
alias venv="uv venv .venv --python 3.11 --allow-existing && source .venv/bin/activate"
alias activate="source .venv/bin/activate"
alias dot='/usr/bin/git --git-dir=$HOME/.dotfiles --work-tree=$HOME'

# Functions
function home() {
  export TEMP_HOME="$PWD"
}

function cd() {
  HOME="${TEMP_HOME:=$HOME}" builtin cd "$@"
}

function rgplace() {
    if [[ $# -lt 2 ]]; then
      echo "Usage: rgplace <search_pattern> <replacement> [file_pattern]"
      echo "Example: rgplace 'foo' 'bar' '*.txt'"
      return 1
    fi

    local search_pattern=$1
    local replacement=$2
    local file_pattern=$3

    if [ -z "$file_pattern" ]; then
      file_pattern="*"
    fi

    rg --color=never --files-with-matches "$search_pattern" --glob "$file_pattern" | while read -r file; do
      sed -i "s|$search_pattern|$replacement|g" "$file"
    done
}

function ga() {
  local message="$1"
  if [ -z "$message" ]; then
    >&2 echo "Commit message is required."
    return 2
  fi
  git commit --amend -m "${message}"
}

function gsp() {
  local subtree="${1:-}"
  shift
  __gsubtree push "$subtree" "$@"
}

function gspull() {
  local subtree="${1}"
  shift
  __gsubtree pull "$subtree" --squash "$@"
}

function __gsubtree() {
  local cmd="${1}"
  shift
  local subtree="${1:-}"
  shift
  local toplevel
  toplevel="$(git rev-parse --show-toplevel)"
  if [ -z "$subtree" ]; then
    >&2 echo "Missing argument 'subtree'."
    echo "Pick one of:"
    # https://stackoverflow.com/a/18339297
    git log | grep git-subtree-dir | tr -d ' ' | cut -d ":" -f2 | sort | uniq | xargs -I {} bash -c 'if [ -d $(git rev-parse --show-toplevel)/{} ] ; then echo "  {}"; fi'
    return 2
  fi
  git -C "$toplevel" subtree "$cmd" --prefix "$subtree" "git@github.com:jmelahman/$(basename "${subtree}").git" master "$@"
}

fixbranch() {
  local branch="$1"
  if [[ -z "$branch" ]]; then
    echo "Usage: fixbranch <branch>"
    return 1
  fi
  git fetch origin "$branch"
  git checkout "$branch"
  prek run --last-commit || true
  git commit -am "nit"
  git push origin "$branch"

  local pr
  pr=$(gh pr list --head "$branch" --json number --jq '.[0].number')
  if [[ -z "$pr" ]]; then
    echo "No open PR found for branch $branch"
    return 1
  fi
  echo "Found PR #$pr"
  gh pr review "$pr" --approve
  gh pr merge "$pr" --auto --squash
}

grb() {
  local input="$1"
  local remote="${input%%:*}"
  local branch="${input#*:}"

  if [[ -z "$remote" || -z "$branch" ]]; then
    echo "Usage: grb user:branch"
    return 1
  fi

  # infer repo URL from origin
  local origin url repo_base new_remote_url

  url=$(git remote get-url origin 2>/dev/null) || {
    echo "Not a git repo or origin missing."
    return 1
  }

  if [[ "$url" =~ github.com[:/](.*)/(.*)(\.git)?$ ]]; then
    repo_base="${match[1]}"
    repo_name="${match[2]}"
  else
    echo "Could not parse GitHub URL from origin: $url"
    return 1
  fi

  new_remote_url="git@github.com:${remote}/${repo_name}"

  # add remote if needed
  if ! git remote get-url "$remote" >/dev/null 2>&1; then
    echo "Adding remote $remote → $new_remote_url"
    git remote add "$remote" "$new_remote_url"
  fi

  echo "Fetching $remote..."
  git fetch "$remote" "$branch"

  echo "Checking out $branch from $remote..."
  git checkout -B "$branch" "$remote/$branch"
}

# Kitty init
KITTY_SHELL_INTEGRATION="${KITTY_INSTALLATION_DIR:=/usr/lib/kitty}/shell-integration/$(basename "${SHELL:-zsh}")/kitty.zsh"
if [ -f "$KITTY_SHELL_INTEGRATION" ]; then
    source "$KITTY_SHELL_INTEGRATION"
elif [ -x "$(command -v kitty)" ]; then
    source <(kitty +kitten shell-integration)
fi

# FZF init
if [ -x "$(command -v fzf)" ] && [ -r /usr/share/fzf/key-bindings.zsh ]
then
    source /usr/share/fzf/key-bindings.zsh
fi

# Load env
if [ -f "$HOME/.env" ]; then
  while read -r line; do
    export "$line"
  done < "$HOME/.env"
fi

# Vim as default
export EDITOR="vim"

# Color LS output to differentiate between directories and files
export LS_OPTIONS="--color=auto"
export CLICOLOR="Yes"
export LSCOLOR=""

export GIT_QUIET=true

# Fall back to less if the configured git pager isn't installed
if ! command -v "$(git config --get core.pager 2>/dev/null | awk '{print $1}')" &>/dev/null; then
  export GIT_PAGER=less
fi

# Customize Path
export GOPATH="$HOME/.go"
export GOBIN="$GOPATH/bin"
export PATH=$HOME/code/monorepo/tools/bin:$HOME/.local/bin:$GOBIN:$HOME/.bun/bin:$PATH:$PATH

export GRIM_DEFAULT_DIR="~/Pictures"

if [ -z "$SSH_AUTH_SOCK" ]; then
  SSH_AUTH_SOCK=$(systemctl --user show-environment | grep SSH_AUTH_SOCK | cut -d= -f2)
  export SSH_AUTH_SOCK=${SSH_AUTH_SOCK:-$XDG_RUNTIME_DIR/ssh-agent.socket}
fi

# https://wiki.archlinux.org/title/Docker#Rootless_Docker_daemon
export DOCKER_HOST="unix://$XDG_RUNTIME_DIR/docker.sock"
export DOCKER_SOCK_PATH="$XDG_RUNTIME_DIR/docker.sock"
if [ -f /.dockerenv ]; then
  export IN_DOCKER=true
else
  export IN_DOCKER=false
  # Load default SSH keys into the agent if it's running but empty
  if [[ -n "$SSH_AUTH_SOCK" ]]; then
    ssh-add -l &>/dev/null || ssh-add &>/dev/null
  fi
fi
export BUILDX_BAKE_ENTITLEMENTS_FS=0

export DEVCONTAINER_FIREWALL=false
export DEVCONTAINER_REMOTE_USER=root
export DEVCONTAINER_REMOTE_HOME=/root
export KANBAN_UID=0
export KANBAN_GID=0

# TODO: npm install changes the lockfile on Linux, https://github.com/onyx-dot-app/onyx/issues/7381
export SKIP=npm-install-check
export IMAGE_TAG=edge

if [ "$IN_DOCKER" != "true" ]; then
  export AWS_PROFILE="jamison"
fi
export HOST_PORT_80="8888"

#export OLLAMA_HOST=http://ollama.home
#export OLLAMA_API_BASE="$OLLAMA_HOST"

# For torch with AMD GPU
export HSA_OVERRIDE_GFX_VERSION=11.0.0

# Added by LM Studio CLI tool (lms)
export PATH="$PATH:/home/jamison/.lmstudio/bin"
