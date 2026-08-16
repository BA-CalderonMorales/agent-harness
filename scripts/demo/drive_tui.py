#!/usr/bin/env python3
"""Scripted pty driver for agent-harness TUI measurements.

Runs the built binary in a pty (TERM=tmux-256color, like the symptom
ledger's real-driver protocol) against scripts/demo/mock-llm.py and
reports the objective metrics:

  K  keypresses from launch to first agent reply rendered
  P  palette entries for a searched command (e.g. /persona)
  D  dead-end strings surfaced in a first-run walk

Usage:
  python3 scripts/demo/drive_tui.py baseline   # first-run wizard walk (K, D)
  python3 scripts/demo/drive_tui.py palette    # palette search count (P)

The mock server must already be running on 127.0.0.1:8080.
"""
import fcntl
import os
import pty
import re
import select
import shutil
import struct
import sys
import termios
import time

MOCK_ENDPOINT = "http://127.0.0.1:8080/v1"

ANSI = re.compile(r"\x1b\[[0-9;?]*[a-zA-Z]|\x1b\][^\x07]*\x07|\x1b[()][0-9A-B]|\r")


def strip_ansi(data: str) -> str:
    return ANSI.sub("", data)


class Driver:
    def __init__(self, env_extra=None):
        self.home = "/tmp/opencode/ah-baseline-home"
        shutil.rmtree(self.home, ignore_errors=True)
        os.makedirs(self.home, exist_ok=True)
        self.cwd = "/tmp/opencode/ah-baseline-cwd"
        shutil.rmtree(self.cwd, ignore_errors=True)
        os.makedirs(self.cwd, exist_ok=True)
        env = os.environ.copy()
        env.update({
            "TERM": "tmux-256color",
            "HOME": self.home,
            "AH_ENDPOINT_URL": MOCK_ENDPOINT,
            "NO_COLOR": "",
        })
        env.pop("AH_PROVIDER", None)
        env.pop("AH_API_KEY", None)
        env.pop("AH_MODEL", None)
        if env_extra:
            env.update(env_extra)
        binary = os.path.abspath("./build/agent-harness")
        self.pid, self.fd = pty.fork()
        if self.pid == 0:
            os.chdir(self.cwd)
            os.environ.clear()
            os.environ.update(env)
            os.execvpe(binary, [binary], env)
        fcntl.ioctl(self.fd, termios.TIOCSWINSZ, struct.pack("HHHH", 40, 120, 0, 0))
        self.buf = ""
        self.frames = []
        time.sleep(0.2)

    def _drain(self, timeout=0.15):
        end = time.time() + timeout
        while time.time() < end:
            r, _, _ = select.select([self.fd], [], [], 0.05)
            if not r:
                continue
            try:
                data = os.read(self.fd, 65536)
            except OSError:
                break
            if not data:
                break
            self.buf += data.decode("utf-8", "replace")

    def read(self, seconds):
        self._drain(seconds)
        return self.snapshot()

    def send(self, text):
        os.write(self.fd, text.encode())

    def keys(self, text, delay=0.25):
        self._drain(0.3)
        for ch in text:
            os.write(self.fd, ch.encode())
            time.sleep(delay)
            self._drain(0.1)

    def snapshot(self):
        self._drain(0.2)
        return strip_ansi(self.buf)

    def frame(self, rows=40):
        """Last full screen (terminal height rows) of the pty buffer."""
        lines = self.snapshot().split("\n")
        return "\n".join(lines[-rows:])

    def wait_for(self, pattern, timeout=20, step=0.25):
        end = time.time() + timeout
        rx = re.compile(pattern)
        while time.time() < end:
            snap = self.snapshot()
            m = rx.search(snap)
            if m:
                return snap, m
            time.sleep(step)
        return snap, None

    def kill(self):
        try:
            os.write(self.fd, b"\x03")
            time.sleep(0.3)
        except OSError:
            pass
        try:
            os.kill(self.pid, 9)
        except OSError:
            pass


def count_in_frames(frames, needle):
    return sum(f.count(needle) for f in frames)


def run_baseline():
    print("== baseline: first-run wizard walk ==")
    d = Driver()
    try:
        snap = d.read(6)
        assert "Setup Required" in snap or "Home" in snap, "did not reach Home"
        print("[home] reached. home banner present:", "Setup Required" in snap)

        # Dead-end scan (D0): strings that describe a problem without affordance
        dead_ends = []
        for probe in ["[! no model]", "Setup Required", "Run /login"]:
            if probe in snap:
                dead_ends.append(probe)
        print("D0 dead-end strings surfaced:", dead_ends)

        keys_pressed = 0
        steps = [
            ("c", 0.8, "chat view"),
            ("i", 0.8, "insert mode"),
            ("/login", 1.2, "typed /login"),
            ("\r", 1.0, "submit command"),
        ]
        for text, wait, label in steps:
            d.keys(text, 0.12)
            snap = d.read(wait)
            keys_pressed += sum(1 for ch in text)
            print(f"  [{label}] keys={keys_pressed}")

        # Provider step: local is index 0 (its default endpoint is
        # http://127.0.0.1:8080/v1, the mock port), Enter picks it.
        snap, m = d.wait_for("choose provider", 5)
        d.keys("\r", 0.5)
        keys_pressed += 1
        print(f"  [provider local] keys={keys_pressed}")

        # Model step (free-text today; Enter accepts default)
        snap, m = d.wait_for("Enter model", 5)
        d.keys("\r", 1.5)
        keys_pressed += 1
        print(f"  [model default] keys={keys_pressed}")

        # Type message and send
        d.keys("hi", 0.3)
        keys_pressed += 2
        d.keys("\r", 1.0)
        keys_pressed += 1
        print(f"  [sent 'hi'] keys={keys_pressed}")

        # Wait for first rendered reply from mock
        snap, m = d.wait_for("Hello! Agent-harness is live", 30, 0.5)
        if m:
            print(f"K0 = {keys_pressed} keypresses to first agent reply")
        else:
            print("K0 = FAILED to reach first agent reply")
            print("--- last snapshot tail ---")
            print(snap[-1200:])
    finally:
        d.kill()


def run_palette():
    print("== palette count (P) ==")
    d = Driver(env_extra={"AH_PROVIDER": "local", "AH_RUNTIME": "llama.cpp",
                          "AH_MODEL": "demo-1.0", "AH_API_KEY": "local"})
    try:
        d.read(6)
        os.write(d.fd, b"\x10")  # Ctrl+P
        snap = d.read(1.0)
        assert "Commands" in snap, "palette did not open"
        d.keys("persona", 0.15)
        snap = d.read(0.8)
        frame = d.frame()
        count = frame.count("/persona")
        print(f"P0: '/persona' appears {count}x in the final palette frame after search")
        print("--- palette frame ---")
        lines = [l for l in frame.split("\n") if l.strip()]
        for l in lines[-25:]:
            print("  " + l)
    finally:
        d.kill()


if __name__ == "__main__":
    mode = sys.argv[1] if len(sys.argv) > 1 else "baseline"
    if mode == "palette":
        run_palette()
    else:
        run_baseline()
