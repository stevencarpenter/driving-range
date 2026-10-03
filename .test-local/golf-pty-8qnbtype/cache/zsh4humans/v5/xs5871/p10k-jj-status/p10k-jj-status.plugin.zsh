setopt localoptions NO_shwordsplit
autoload -Uz VCS_INFO_bydir_detect

typeset -g POWERLEVEL9K_JJ_STATUS_COMMAND=(
    jj log --ignore-working-copy --color always --revisions @ --no-graph --template '
    if(root,
      format_root_commit(self),
      concat(
        separate(" ",
          if(immutable, label("immutable", "◆")),
          if(conflict, label("conflict", "×")),
          format_short_change_id_with_hidden_and_divergent_info(self),
          truncate_end(18, bookmarks, "…"),
          tags,
          working_copies,
          if(config("ui.show-cryptographic-signatures").as_boolean(),
            format_short_cryptographic_signature(signature)),
          if(empty, label("empty", "∅")),
          if(description,
            truncate_end(24, description.first_line(), "…"),
            label(if(empty, "empty", "description placeholder"), "#"),
          ),
        ),
        "\n"
      ),
    )')

function prompt_jj_status() {
    local -A vcs_comm
    vcs_comm[detect_need_file]=working_copy

    VCS_INFO_bydir_detect .jj \
    && p10k segment -t \
        "$(${POWERLEVEL9K_JJ_STATUS_COMMAND[@]} \
            | tr -d '\n' \
            | sed -r "s/\[[^m]+m/%{&%}/g")"
}
