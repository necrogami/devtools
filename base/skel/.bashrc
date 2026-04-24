# ~/.bashrc — interactive shell config for the `dev` user inside devtools.
# Seeded from /etc/skel on first container start.

# Bail for non-interactive shells (scp, rsync, docker exec one-shots).
case $- in
    *i*) ;;
      *) return ;;
esac

# ----- history ----------------------------------------------------------------
HISTCONTROL=ignoreboth:erasedups
HISTSIZE=100000
HISTFILESIZE=200000
HISTTIMEFORMAT='%F %T  '
shopt -s histappend
# Append after every command AND re-read from disk so multiple sessions share.
PROMPT_COMMAND="history -a; history -n; ${PROMPT_COMMAND:-}"

# ----- shell options ----------------------------------------------------------
shopt -s autocd cdspell dirspell globstar checkwinsize no_empty_cmd_completion

# ----- completions ------------------------------------------------------------
if ! shopt -oq posix; then
    if [ -f /usr/share/bash-completion/bash_completion ]; then
        . /usr/share/bash-completion/bash_completion
    elif [ -f /etc/bash_completion ]; then
        . /etc/bash_completion
    fi
fi

# ----- PATH -------------------------------------------------------------------
export PATH="$HOME/.local/bin:$PATH"

# NOTE: the oh-my-posh prompt, fzf key-bindings, and Homebrew shellenv are
# all wired up system-wide via /etc/profile.d/ (devtools-shell.sh +
# brew.sh), sourced by /etc/bash.bashrc for non-login interactive shells
# and by /etc/profile for login shells. Keeping them out of this file
# means upgrades to the init never require users to diff or re-seed their
# ~/.bashrc.

# ----- aliases ----------------------------------------------------------------
[ -f "$HOME/.bash_aliases" ] && . "$HOME/.bash_aliases"

# ----- editor -----------------------------------------------------------------
export EDITOR="${EDITOR:-vim}"
export VISUAL="${VISUAL:-$EDITOR}"
export PAGER="${PAGER:-less}"
export LESS="${LESS:--R}"
