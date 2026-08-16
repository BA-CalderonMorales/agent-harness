#!/usr/bin/env python3
"""Scripted tmux driver for agent-harness TUI measurements.

Runs the built binary in a tmux pane (TERM=tmux-256color, the symptom
ledger's real-driver protocol) against scripts/demo/mock-llm.py and
reports the objective metrics:

  K  keypresses from launch to first agent reply rendered
  P  palette entries for a searched command (e.g. /persona)
  D  dead-end strings surfaced in a first-run walk

Screen truth is read via `tmux capture-pane` (the pty byte stream only
shows the renderer's differential updates, not the screen).

Usage:
  python3 scripts/demo/drive_tui.py baseline   # first-run wizard walk (K, D)
  python3 scripts/demo/drive_tui.py palette    # palette search count (P)

The mock server must already be running on 127.0.0.1:8080.
"""
import os
import re
import shutil
import subprocess
import sys
import time

MOCK_ENDPOINT = "http://127.0.0.1:8080/v1"
SESSION = "ah-drive"
HOME = "/tmp/opencode/ah-drive-home"
CWD = "/tmp/opencode/ah-drive-cwd"


def run(args, **kw):
    return subprocess.run(args, capture_output=True, text=True, **kw)


class Driver:
    def __init__(self, env_extra=None, rows=40, cols=120):
        shutil.rmtree(HOME, ignore_errors=True)
        shutil.rmtree(CWD, ignore_errors=True)
        os.makedirs(HOME, exist_ok=True)
        os.makedirs(CWD, exist_ok=True)
        env = os.environ.copy()
        env.update({"TERM": "tmux-256color", "HOME": HOME,
                    "AH_ENDPOINT_URL": MOCK_ENDPOINT})
        env.pop("AH_PROVIDER", None)
        env.pop("AH_API_KEY", None)
        env.pop("AH_MODEL", None)
        if env_extra:
            env.update(env_extra)
        envstr = " ".join(f"{k}={shlex_quote(v)}" for k, v in env.items())
        binary = os.path.abspath("./build/agent-harness")
        run(["tmux", "-f", "/dev/null", "kill-session", "-t", SESSION])
        cmd = f"cd {shlex_quote(CWD)} && {envstr} {shlex_quote(binary)}"
        run(["tmux", "-f", "/dev/null", "new-session", "-d", "-s", SESSION,
             "-x", str(cols), "-y", str(rows), cmd])
        time.sleep(0.3)

    def screen(self):
        p = run(["tmux", "-f", "/dev/null", "capture-pane", "-t", SESSION, "-p"])
        return p.stdout

    def send(self, text, delay=0.12):
        for ch in text:
            if ch == "\r":
                run(["tmux", "-f", "/dev/null", "send-keys", "-t", SESSION, "Enter"])
            elif ch == "\x10":
                run(["tmux", "-f", "/dev/null", "send-keys", "-t", SESSION, "C-p"])
            else:
                run(["tmux", "-f", "/dev/null", "send-keys", "-t", SESSION, ch])
            time.sleep(delay)

    def wait_for(self, pattern, timeout=25, step=0.25):
        rx = re.compile(pattern)
        end = time.time() + timeout
        while time.time() < end:
            snap = self.screen()
            m = rx.search(snap)
            if m:
                return snap, m
            time.sleep(step)
        return snap, None

    def kill(self):
        run(["tmux", "-f", "/dev/null", "kill-session", "-t", SESSION])


def shlex_quote(s):
    return "'" + s.replace("'", "'\\''") + "'"


def run_baseline():
    print("== baseline: first-run wizard walk ==")
    d = Driver()
    try:
        snap, _ = d.wait_for(r"Home|Setup Required", timeout=15)
        assert "Home" in snap or "Setup Required" in snap, "did not reach Home"
        print("[home] reached. setup banner present:", "Setup Required" in snap)

        dead_ends = []
        for probe in ["[! no model]", "Setup Required", "Run /login"]:
            if probe in snap:
                dead_ends.append(probe)
        print("D0 dead-end strings surfaced:", dead_ends)

        keys_pressed = 0
        for text, wait, label in [
            ("c", 0.8, "chat view"),
            ("i", 0.8, "insert mode"),
            ("/login", 1.0, "typed /login"),
            ("\r", 1.0, "submit command"),
        ]:
            d.send(text)
            time.sleep(wait)
            keys_pressed += len(text)
            print(f"  [{label}] keys={keys_pressed}")

        d.send("\r", 0.5)  # provider: local (index 0)
        keys_pressed += 1
        print(f"  [provider local] keys={keys_pressed}")
        time.sleep(1.0)

        d.send("\r", 0.8)  # model: Enter accepts default
        keys_pressed += 1
        print(f"  [model default] keys={keys_pressed}")
        time.sleep(1.5)

        d.send("hi")
        keys_pressed += 2
        d.send("\r", 0.8)
        keys_pressed += 1
        print(f"  [sent 'hi'] keys={keys_pressed}")

        snap, m = d.wait_for("Hello! Agent-harness is live")
        if m:
            print(f"K0 = {keys_pressed} keypresses to first agent reply")
        else:
            print("K0 = FAILED to reach first agent reply")
            print(snap[-800:])
    finally:
        d.kill()


def run_palette():
    print("== palette count (P) ==")
    d = Driver(env_extra={"AH_PROVIDER": "local", "AH_RUNTIME": "llama.cpp",
                          "AH_MODEL": "demo-1.0", "AH_API_KEY": "local"})
    try:
        time.sleep(6)
        d.send("\x10", 0.5)  # Ctrl+P
        time.sleep(1.0)
        assert "Commands" in d.screen(), "palette did not open"
        d.send("persona")
        time.sleep(1.0)
        snap = d.screen()
        count = snap.count("/persona")
        print(f"P: '/persona' appears {count}x in the palette after search")
        print("--- palette lines ---")
        for line in snap.split("\n"):
            if line.strip():
                print("  " + line.strip())
    finally:
        d.kill()


if __name__ == "__main__":
    mode = sys.argv[1] if len(sys.argv) > 1 else "baseline"
    if mode == "palette":
        run_palette()
    else:
        run_baseline()
