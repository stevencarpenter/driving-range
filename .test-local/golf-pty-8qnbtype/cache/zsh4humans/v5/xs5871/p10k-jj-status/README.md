# p10k jj status

## What?

A zsh / powerlevel10k plugin for displaying the status of jujutsu repositories
on the command line prompt.
It supports jj's native color output and doesn't use vcs_info.

## Why?

1. Something in the vcs_info + powerlevel10k's vcs segment (the one used for
   gitstatus) breaks escaping of escape sequences, which is necessary for a
   properly aligned right side prompt.
2. In colocated repositories, it'd be nice to also see info from gitstatus.

## How?

Works like any other zsh plugin.
Either use a plugin manager and copy/clone

```zsh
# .zshrc

# Example: oh my zsh
plugins=(
    # other plugins...
    p10k-jj-status
)

# Example: manual
source ~/path-to-copy/p10k-jj-status.plugin.zsh
```

There's one (1) configuration variable `POWERLEVEL9K_JJ_STATUS_COMMAND`.
The default command is a slightly modified version of jj's
`template-aliases.builtin_log_oneline` with some boilerplate command line
arguments.
It works out-of-the-box without additional jj configuration.
If customized, it is probably a good idea to put the template and a command
alias into the jj config.
