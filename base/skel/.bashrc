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

# ----- mise (runtime manager) -------------------------------------------------
# profile.d also activates mise for login shells; re-activate for non-login
# interactive shells so `docker exec -it ... bash` (no -l) gets shims + hooks.
if [ -x /usr/local/bin/mise ]; then
    eval "$(/usr/local/bin/mise activate bash)"
fi

# ----- oh-my-posh prompt (Atomic) --------------------------------------------
if command -v oh-my-posh >/dev/null 2>&1; then
    omp_theme="${POSH_THEMES_PATH:-/usr/local/share/oh-my-posh/themes}/atomic.omp.json"
    [ -f "$omp_theme" ] || omp_theme="$HOME/.config/oh-my-posh/atomic.omp.json"
    if [ -f "$omp_theme" ]; then
        eval "$(oh-my-posh init bash --config "$omp_theme")"
    fi
    unset omp_theme
fi

# ----- aliases ----------------------------------------------------------------
[ -f "$HOME/.bash_aliases" ] && . "$HOME/.bash_aliases"

# ----- editor -----------------------------------------------------------------
export EDITOR="${EDITOR:-vim}"
export VISUAL="${VISUAL:-$EDITOR}"
export PAGER="${PAGER:-less}"
export LESS="${LESS:--R}"

# ----- fzf (Debian package) ---------------------------------------------------
[ -f /usr/share/doc/fzf/examples/key-bindings.bash ] && . /usr/share/doc/fzf/examples/key-bindings.bash
[ -f /usr/share/bash-completion/completions/fzf ]    && . /usr/share/bash-completion/completions/fzf
