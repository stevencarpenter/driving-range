#!/usr/bin/env python3
"""Real PTY smoke: terminal restoration, saved attempts, signals, native editors.

Run after `just build` and `golf setup`: python3 scripts/smoke.py.
Uses Python's standard library and only temporary state/owned Docker volumes.
"""

import fcntl
import json
import os
from pathlib import Path
import pty
import select
import signal
import struct
import subprocess
import tempfile
import termios
import time


ROOT = Path(__file__).resolve().parents[1]
BINARY = ROOT / "golf"
PRACTICE_ENV = {}


class Terminal:
    def __init__(self, args, state):
        self.master, self.slave = pty.openpty()
        self.initial = termios.tcgetattr(self.slave)
        self.resize(24, 80)
        self.process = subprocess.Popen(
            [str(BINARY), "--state-dir", str(state), *args],
            stdin=self.slave, stdout=self.slave, stderr=self.slave,
            env=dict(os.environ, TERM="xterm-256color", NO_COLOR="1", **PRACTICE_ENV),
            start_new_session=True,
        )
        self.output = b""

    def resize(self, rows, columns):
        fcntl.ioctl(self.slave, termios.TIOCSWINSZ, struct.pack("HHHH", rows, columns, 0, 0))
        if hasattr(self, "process"):
            os.kill(self.process.pid, signal.SIGWINCH)

    def send(self, data):
        os.write(self.master, data)

    def wait_text(self, text, seconds=30):
        deadline = time.monotonic() + seconds
        while text not in self.output:
            if time.monotonic() >= deadline:
                raise AssertionError(f"Missing {text!r}: {self.output[-5000:]!r}")
            ready, _, _ = select.select([self.master], [], [], 0.1)
            if ready:
                try:
                    self.output += os.read(self.master, 65536)
                except OSError:
                    break
            if self.process.poll() is not None and not ready:
                break
        assert text in self.output, (text, self.output[-5000:])

    def finish(self, expected=0):
        deadline = time.monotonic() + 30
        while self.process.poll() is None and time.monotonic() < deadline:
            if select.select([self.master], [], [], 0.1)[0]:
                self.output += os.read(self.master, 65536)
        assert self.process.poll() is not None, ("process did not finish", self.output[-5000:])
        assert self.process.returncode == expected, self.output[-5000:]
        final = termios.tcgetattr(self.slave)
        mask = termios.ECHO | termios.ICANON | termios.ISIG
        assert final[3] & mask == self.initial[3] & mask, "terminal mode not restored"

    def close(self):
        if self.process.poll() is None:
            self.process.terminate()
            try:
                self.process.wait(timeout=3)
            except subprocess.TimeoutExpired:
                self.process.kill()
                self.process.wait(timeout=3)
        os.close(self.master)
        os.close(self.slave)


def plain(state, *args):
    return subprocess.check_output([str(BINARY), "--state-dir", str(state), *args], text=True)


def session(state, args, input_bytes, expected):
    terminal = Terminal(args, state)
    try:
        terminal.wait_text(b"Exit to check your work")
        terminal.resize(30, 100)
        terminal.send(input_bytes)
        terminal.finish(expected)
        return terminal.output
    finally:
        terminal.close()


def main():
    assert BINARY.is_file(), "Run just build first"
    with tempfile.TemporaryDirectory(prefix="golf-pty-") as directory:
        state = Path(directory)
        config = state / "config" / "golf-smoke"
        config.mkdir(parents=True)
        (config / "init.lua").write_text(
            'vim.keymap.set("n", "Q", ":%s/port=3000/port=8080/<CR>:wq<CR>")\n'
        )
        PRACTICE_ENV.update(
            XDG_CONFIG_HOME=str(state / "config"),
            XDG_DATA_HOME=str(state / "data"),
            XDG_STATE_HOME=str(state / "editor-state"),
            XDG_CACHE_HOME=str(state / "cache"),
            NVIM_APPNAME="golf-smoke",
        )
        try:
            session(state, ["play", "search.error-records"], b"exit\n", 1)
            first = json.loads(plain(state, "export"))["attempts"][0]
            assert first["status"] == "active", first
            command = b"printf '%s\\n' \"grep '^ERROR' app.log\" > solution.sh\nexit\n"
            session(state, ["play", first["id"]], command, 0)
            result = json.loads(plain(state, "export"))
            assert len(result["attempts"]) == 1 and len(result["sessions"]) == 2
            assert result["attempts"][0]["status"] == "solved"
            assert [c["result"]["outcome"] for c in result["checks"]] == ["fail", "pass"]
            session(state, ["retry", first["id"]], b"exit\n", 1)
            attempts = json.loads(plain(state, "export"))["attempts"]
            assert len(attempts) == 2 and len({a["workspace"] for a in attempts}) == 2
            terminal = Terminal(["play", "search.count-matches"], state)
            try:
                terminal.wait_text(b"Exit to check your work")
                terminal.process.send_signal(signal.SIGTERM)
                terminal.finish(1)
            finally:
                terminal.close()
            interrupted = json.loads(plain(state, "export"))
            attempt = next(a for a in interrupted["attempts"] if a["exercise_id"] == "search.count-matches")
            assert attempt["status"] == "interrupted", attempt
            assert not any(c["attempt_id"] == attempt["id"] for c in interrupted["checks"])
            plain(state, "config", "track", "vim")
            terminal = Terminal([], state)
            try:
                terminal.wait_text(b"Today")
                terminal.output = b""
                terminal.send(b"2")
                terminal.wait_text(b"PRACTICE")
                terminal.resize(20, 60)
                terminal.output = b""
                terminal.send(b"?")
                terminal.wait_text(b"KEYBOARD")
                terminal.output = b""
                terminal.send(b"?")
                terminal.wait_text(b"PRACTICE")
                terminal.resize(24, 80)
                terminal.output = b""
                terminal.send(b"/")
                # The search cursor may be emitted separately from its label.
                terminal.wait_text(b"Type to filter   Ctrl-U clear   Enter / Esc finish")
                terminal.output = b""
                terminal.send(b"vim.change-value\r")
                # Query text may arrive as several cursor-positioned updates.
                # Wait for Enter to leave search mode, then verify the detail.
                terminal.wait_text(b"j/k move   / search   Enter open")
                terminal.output = b""
                terminal.send(b"\r")
                terminal.wait_text(b"Change one configuration value")
                terminal.output = b""
                terminal.send(b"\r")
                # The objective names port=3000, so wait for pane content that only
                # the rendered fixture has. Waiting on "port=3000" matches the
                # preparing screen and sends the key before the child exists.
                terminal.wait_text(b"host=localhost")
                terminal.send(b"Q")
                terminal.wait_text(b"PASS")
                terminal.send(b"q")
                terminal.finish(0)
            finally:
                terminal.close()
            results = json.loads(plain(state, "export"))
            assert any(a["exercise_id"] == "vim.change-value" and a["status"] == "solved" for a in results["attempts"])
            zsh_config = state / "zsh"
            zsh_config.mkdir()
            (zsh_config / ".zshrc").write_text(
                "alias golf-zsh-config='print -r -- GOLF_ZSH_CONFIG_LOADED'\n"
            )
            PRACTICE_ENV["ZDOTDIR"] = str(zsh_config)
            for exercise in ("zsh.array-boundaries", "zsh.glob-qualifier"):
                output = session(state, ["play", exercise], b"golf-zsh-config\nexit\n", 1)
                assert b"GOLF_ZSH_CONFIG_LOADED" in output, output[-5000:]
            print("PASS: shell failure/resume/retry, SIGTERM persistence, TUI resize and native Neovim custom keybinding, zsh challenges with .zshrc alias, terminal restoration")
        finally:
            # IDs come exclusively from this test's temporary database.
            if (state / "driving-range.db").exists():
                for attempt in json.loads(plain(state, "export"))["attempts"]:
                    if attempt["status"] not in ("solved", "abandoned"):
                        plain(state, "abandon", attempt["id"], "--yes")
                    plain(state, "forget", attempt["id"], "--yes")


if __name__ == "__main__":
    main()
