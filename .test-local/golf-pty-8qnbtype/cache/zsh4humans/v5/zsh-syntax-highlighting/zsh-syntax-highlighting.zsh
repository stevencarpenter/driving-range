# -------------------------------------------------------------------------------------------------
# Copyright (c) 2010-2020 zsh-syntax-highlighting contributors
# All rights reserved.
#
# Redistribution and use in source and binary forms, with or without modification, are permitted
# provided that the following conditions are met:
#
#  * Redistributions of source code must retain the above copyright notice, this list of conditions
#    and the following disclaimer.
#  * Redistributions in binary form must reproduce the above copyright notice, this list of
#    conditions and the following disclaimer in the documentation and/or other materials provided
#    with the distribution.
#  * Neither the name of the zsh-syntax-highlighting contributors nor the names of its contributors
#    may be used to endorse or promote products derived from this software without specific prior
#    written permission.
#
# THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS" AND ANY EXPRESS OR
# IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND
# FITNESS FOR A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR
# CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL
# DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
# DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER
# IN CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT
# OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
# -------------------------------------------------------------------------------------------------
# -*- mode: zsh; sh-indentation: 2; indent-tabs-mode: nil; sh-basic-offset: 2; -*-
# vim: ft=zsh sw=2 ts=2 et
# -------------------------------------------------------------------------------------------------

typeset -g ZSH_HIGHLIGHT_VERSION=z4h
typeset -g ZSH_HIGHLIGHT_REVISION=HEAD

# -------------------------------------------------------------------------------------------------
# Core highlighting update system
# -------------------------------------------------------------------------------------------------

# Array declaring active highlighters names.
typeset -ga ZSH_HIGHLIGHT_HIGHLIGHTERS

# Update ZLE buffer syntax highlighting.
#
# Invokes each highlighter that needs updating.
# This function is supposed to be called whenever the ZLE state changes.
_zsh_highlight()
{
  # Store the previous command return code to restore it whatever happens.
  local ret=$?
  # Make it read-only.  Can't combine this with the previous line when POSIX_BUILTINS may be set.
  typeset -r ret

  # Remove all highlighting in isearch, so that only the underlining done by zsh itself remains.
  # For details see FAQ entry 'Why does syntax highlighting not work while searching history?'.
  # This disables highlighting during isearch (for reasons explained in README.md) unless zsh is new enough
  # and doesn't have the pattern matching bug
  if [[ $WIDGET == zle-isearch-update && -v ISEARCHMATCH_ACTIVE ]]; then
    region_highlight=()
    return ret
  fi

  # Before we 'emulate -L', save the user's options
  if [[ -n ${ZSH_HIGHLIGHT_HIGHLIGHTERS:#(brackets|cursor|line|main|pattern|regexp|root)} ]]; then
    # Copy all options if there are user-defined highlighters
    local -rA zsyh_user_options=("${(kv)options[@]}")
  else
    # Copy a subset of options used by the bundled highlighters.  This is faster than
    # copying all options.
    local -rA zsyh_user_options=(
      ignorebraces        "${options[ignorebraces]}"
      ignoreclosebraces   "${options[ignoreclosebraces]-off}"
      pathdirs            "${options[pathdirs]}"
      interactivecomments "${options[interactivecomments]}"
      globassign          "${options[globassign]}"
      multifuncdef        "${options[multifuncdef]}"
      autocd              "${options[autocd]}"
      equals              "${options[equals]}"
      multios             "${options[multios]}"
      rcquotes            "${options[rcquotes]}"
    )
  fi

  local -a new_highlight

  emulate -L zsh -o warncreateglobal -o nobashrematch
  local REPLY # don't leak $REPLY into global scope

  {
    local cache_place highlighter

    # Select which highlighters in ZSH_HIGHLIGHT_HIGHLIGHTERS need to be invoked.
    for highlighter in $ZSH_HIGHLIGHT_HIGHLIGHTERS; do
      # eval cache place for current highlighter and prepare it
      cache_place="_zsh_highlight__highlighter_${highlighter}_cache"
      typeset -ga ${cache_place}

      # If highlighter needs to be invoked
      if "_zsh_highlight_highlighter_${highlighter}_predicate"; then
        # Execute highlighter and save result
        () {
          # Manipulating zle's region_highlight directly is slow, so we hide it
          # with a regular array.
          local -ah region_highlight=()
          "_zsh_highlight_highlighter_${highlighter}_paint"
          : ${(AP)cache_place::="${region_highlight[@]}"}
          new_highlight+=($region_highlight)
        }
      else
        # Use value form cache if any cached
        new_highlight+=("${(@P)cache_place}")
      fi
    done

    region_highlight=($new_highlight)

    # Re-apply zle_highlight settings

    # region
    if (( REGION_ACTIVE )); then
      if (( MARK > CURSOR )) ; then
        integer min=CURSOR max=MARK
      else
        integer min=MARK max=CURSOR
      fi
      if (( REGION_ACTIVE == 1 )); then
        [[ $KEYMAP = vicmd ]] && (( ++max ))
      elif (( REGION_ACTIVE == 2 )); then
        local needle=$'\n'
        # CURSOR and MARK are 0 indexed between letters like region_highlight
        # Do not include the newline in the highlight
        (( min = ${BUFFER[(Ib:min:)$needle]} ))
        (( max = ${BUFFER[(ib:max:)$needle]} - 1 ))
      fi
      _zsh_highlight_apply_zle_highlight region standout $min $max
    fi

    # yank / paste (zsh-5.1.1 and newer)
    (( YANK_ACTIVE )) && _zsh_highlight_apply_zle_highlight paste standout "$YANK_START" "$YANK_END"

    # isearch
    (( ISEARCHMATCH_ACTIVE )) && _zsh_highlight_apply_zle_highlight isearch underline "$ISEARCHMATCH_START" "$ISEARCHMATCH_END"

    # suffix
    (( SUFFIX_ACTIVE )) && _zsh_highlight_apply_zle_highlight suffix bold "$SUFFIX_START" "$SUFFIX_END"

    return ret

  } always {
    typeset -g _ZSH_HIGHLIGHT_PRIOR_BUFFER="$BUFFER"
    typeset -gi _ZSH_HIGHLIGHT_PRIOR_CURSOR=CURSOR
  }
}

# Apply highlighting based on entries in the zle_highlight array.
# This function takes four arguments:
# 1. The exact entry (no patterns) in the zle_highlight array:
#    region, paste, isearch, or suffix
# 2. The default highlighting that should be applied if the entry is unset
# 3. and 4. Two integer values describing the beginning and end of the
#    range. The order does not matter.
_zsh_highlight_apply_zle_highlight() {
  local entry="$1" default="$2"
  integer first="$3" second="$4"

  # read the relevant entry from zle_highlight
  #
  # ### In zsh≥5.0.8 we'd use ${(b)entry}, but we support older zsh's, so we don't
  # ### add (b).  The only effect is on the failure mode for callers that violate
  # ### the precondition.
  local region="${zle_highlight[(r)${entry}:*]-}"

  if [[ -z "$region" ]]; then
    # entry not specified at all, use default value
    region=$default
  else
    # strip prefix
    region="${region#${entry}:}"

    # no highlighting when set to the empty string or to 'none'
    if [[ -z "$region" || "$region" == none ]]; then
      return
    fi
  fi

  if (( first < second )); then
    region_highlight+=("$first $second $region")
  else
    region_highlight+=("$second $first $region")
  fi
}


# -------------------------------------------------------------------------------------------------
# API/utility functions for highlighters
# -------------------------------------------------------------------------------------------------

# Array used by highlighters to declare user overridable styles.
typeset -gA ZSH_HIGHLIGHT_STYLES

# Whether the command line buffer has been modified or not.
#
# Returns 0 if the buffer has changed since _zsh_highlight was last called.
_zsh_highlight_buffer_modified()
{
  [[ ${#_ZSH_HIGHLIGHT_PRIOR_BUFFER} -ne ${#BUFFER} || ${_ZSH_HIGHLIGHT_PRIOR_BUFFER:-} != $BUFFER ]]
}

# Whether the cursor has moved or not.
#
# Returns 0 if the cursor has moved since _zsh_highlight was last called.
_zsh_highlight_cursor_moved()
{
  [[ ${CURSOR:--1} -ne ${_ZSH_HIGHLIGHT_PRIOR_CURSOR:--2} ]]
}

# Add a highlight defined by ZSH_HIGHLIGHT_STYLES.
#
# Should be used by all highlighters aside from 'pattern' (cf. ZSH_HIGHLIGHT_PATTERN).
# Overwritten in tests/test-highlighting.zsh when testing.
_zsh_highlight_add_highlight()
{
  local -i start end
  local highlight
  start=$1
  end=$2
  shift 2
  for highlight; do
    if (( $+ZSH_HIGHLIGHT_STYLES[$highlight] )); then
      region_highlight+=("$start $end $ZSH_HIGHLIGHT_STYLES[$highlight]")
      break
    fi
  done
}

[[ -v ZSH_HIGHLIGHT_HIGHLIGHTERS ]] || ZSH_HIGHLIGHT_HIGHLIGHTERS=(main)

() {
  setopt localoptions noksharrays bareglobqual
  local highlighter_dir
  for highlighter_dir in $1/${^ZSH_HIGHLIGHT_HIGHLIGHTERS}; do
    . "$highlighter_dir/${highlighter_dir:t}-highlighter.zsh"
  done
} "${${(%):-%N}:h}/highlighters"
