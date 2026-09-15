#!/bin/sh
# Exercise the installer with fixture executables, without rebuilding Go for each case.
set -eu
scratch=$(mktemp -d)
trap 'rm -rf "$scratch"' 0
cp justfile "$scratch/justfile"
export GOLF_INSTALL_DIR="$scratch/install dir/bin"

run_install() {
    just --justfile "$scratch/justfile" GO=true install >"$scratch/output" 2>&1
}

printf '#!/bin/sh\necho golf old\n' >"$scratch/golf"
run_install
[ "$("$GOLF_INSTALL_DIR/golf" --version)" = 'golf old' ]
# The previous inode must survive replacement for processes already using it.
ln "$GOLF_INSTALL_DIR/golf" "$scratch/old-golf"
printf '#!/bin/sh\necho golf new\n' >"$scratch/golf"
run_install
[ "$("$GOLF_INSTALL_DIR/golf" --version)" = 'golf new' ]
[ "$("$scratch/old-golf" --version)" = 'golf old' ]
run_install
[ "$("$GOLF_INSTALL_DIR/golf" --version)" = 'golf new' ]

if just --justfile "$scratch/justfile" GO=false install >"$scratch/output" 2>&1; then
    echo 'Expected build failure' >&2
    exit 1
fi
[ "$("$GOLF_INSTALL_DIR/golf" --version)" = 'golf new' ]
rm "$scratch/golf"
if run_install; then
    echo 'Expected copy failure' >&2
    exit 1
fi
[ "$("$GOLF_INSTALL_DIR/golf" --version)" = 'golf new' ]
set -- "$GOLF_INSTALL_DIR"/.golf.*
[ ! -e "$1" ]

printf '#!/bin/sh\necho golf latest\n' >"$scratch/golf"
run_install
case "$(cat "$scratch/output")" in
    *'Put '*' first on PATH'*) ;;
    *) echo 'Missing PATH guidance' >&2; exit 1 ;;
esac
PATH="$GOLF_INSTALL_DIR:$PATH" run_install
case "$(cat "$scratch/output")" in
    *'first on PATH'*) echo 'Unexpected PATH guidance' >&2; exit 1 ;;
esac
mv "$GOLF_INSTALL_DIR/golf" "$scratch/installed"
mkdir "$GOLF_INSTALL_DIR/golf"
if run_install; then
    echo 'Expected rejection of a directory named golf' >&2
    exit 1
fi
[ -z "$(ls -A "$GOLF_INSTALL_DIR/golf")" ]
printf 'PASS: fresh install, upgrade, repeat install, failure recovery, PATH guidance\n'
