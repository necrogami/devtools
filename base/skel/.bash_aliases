# ~/.bash_aliases — pulled in by ~/.bashrc
# Keep personal project-specific aliases in ~/.bash_aliases.local.

# ----- listing (eza as ls replacement) ---------------------------------------
alias ls='eza --icons=auto --group-directories-first'
alias l='eza -lh --icons=auto --group-directories-first'
alias ll='eza -lah --icons=auto --group-directories-first'
alias la='eza -lah --icons=auto --group-directories-first'
alias tree='eza --tree --icons=auto'

# ----- viewers ---------------------------------------------------------------
alias cat='bat --paging=never --style=plain'
alias cata='bat'            # full bat with paging + line numbers

# ----- git -------------------------------------------------------------------
alias g='git'
alias gs='git status'
alias gl='git log --oneline --graph --decorate -20'
alias gla='git log --oneline --graph --decorate --all -30'
alias gd='git diff'
alias gdc='git diff --cached'
alias gc='git commit'
alias gca='git commit --amend'
alias gco='git checkout'
alias gb='git branch'
alias gsw='git switch'
alias gst='git stash'

# ----- docker ----------------------------------------------------------------
alias dc='docker compose'
alias dps='docker ps --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}"'

# ----- k8s -------------------------------------------------------------------
alias k='kubectl'

# ----- safety nets -----------------------------------------------------------
alias rm='rm -i'
alias cp='cp -i'
alias mv='mv -i'

# ----- navigation ------------------------------------------------------------
alias ..='cd ..'
alias ...='cd ../..'
alias ....='cd ../../..'

# Load machine-local extras if present (never committed).
[ -f "$HOME/.bash_aliases.local" ] && . "$HOME/.bash_aliases.local"
