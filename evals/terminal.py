"""Drive the real TUI in a PTY, with bounded fixture approvals and evidence.

Linux only. Bubblewrap is mandatory: model-written code and the independent
verifier see the fixture, not the user's home. Network remains available for
the local model endpoint. This is not a general network isolation policy.
"""
import errno
import fcntl
import json
import os
from pathlib import Path
import pty
import re
import select
import shutil
import shlex
import signal
import struct
import subprocess
import termios
import tempfile
import time
from urllib.parse import urlsplit
from resources import Meter, profile, command_prefix
from protocol import parse_event


def sandbox(case_dir, command, network=True):
    """Expose system programs and this case; never mount the user's home."""
    case_dir = Path(case_dir).resolve()
    args = ["bwrap", "--die-with-parent", "--unshare-pid", "--new-session",
            "--ro-bind", "/usr", "/usr",
            "--proc", "/proc", "--dev", "/dev", "--tmpfs", "/tmp"]
    if not network:
        args += ["--unshare-net"]
    for name in ("bin", "sbin", "lib", "lib64"):
        path = Path("/" + name)
        if path.is_symlink():
            args += ["--symlink", os.readlink(path), str(path)]
        elif path.exists():
            args += ["--ro-bind", str(path), str(path)]
    for name in ("passwd", "group", "nsswitch.conf", "hosts", "resolv.conf", "localtime"):
        path = "/etc/" + name
        if Path(path).exists():
            args += ["--ro-bind", path, path]
    # On Arch, /etc/ssl/cert.pem points outside /etc/ssl. Preserve that
    # target too; otherwise isolation fabricates certificate failures.
    for path in ("/etc/ssl", "/etc/ca-certificates", "/etc/pki"):
        if Path(path).exists():
            args += ["--ro-bind", path, path]
    # Mount only mutable application data, never host-owned reports or graders.
    for name in ("workspace", "home", "config", "data", "sessions"):
        path = case_dir / name
        if path.is_dir() and not path.is_symlink():
            args += ["--bind", str(path), str(path)]
    for name in ("agent-harness", ".event-pipe"):
        path = case_dir / name
        if path.exists() and not path.is_symlink():
            args += ["--ro-bind", str(path), str(path)]
    return args + ["--chdir", str(case_dir / "workspace"), "--", *map(str, command)]


def available():
    if not shutil.which("bwrap"):
        raise RuntimeError("unattended terminal runs require Linux bubblewrap")
    with tempfile.TemporaryDirectory(prefix="ah-sandbox-probe-") as directory:
        (Path(directory) / "workspace").mkdir()
        probe = subprocess.run(sandbox(directory, ["/usr/bin/true"]), capture_output=True, timeout=5)
    if probe.returncode:
        raise RuntimeError("bubblewrap is unavailable: " + probe.stderr.decode(errors="replace")[:300])


def allowed(tool, inputs, workspace, coding, weather=False, implementation="names.py", test_module="test_names"):
    """Authorize only the actions specified by the coding/recovery fixture."""
    if not isinstance(inputs, dict):
        return False
    if weather and tool == "web_fetch":
        try:
            url = urlsplit(inputs.get("url", ""))
            return (url.scheme == "https" and url.port in {None, 443} and not url.username
                    and not url.password and url.hostname in {
                        "api.weather.gov", "forecast.weather.gov", "www.weather.gov", "weather.gov",
                        "www.accuweather.com", "weather.com", "www.weather.com",
                        "api.open-meteo.com", "wttr.in"})
        except (TypeError, ValueError):
            return False
    if not coding:
        return False
    if tool in {"read", "glob", "grep", "ls", "ls_recursive", "search"}:
        return True  # The OS boundary contains all readable user data.
    if tool in {"write", "edit"}:
        value = inputs.get("file_path", "")
        if not isinstance(value, str) or not value:
            return False
        path = Path(value)
        if not path.is_absolute():
            path = workspace / path
        try:
            return path.resolve() == workspace.resolve() / implementation
        except (OSError, RuntimeError):
            return False
    if tool == "bash":
        return test_command(inputs.get("command"), workspace, test_module)
    return False


def test_command(command, workspace, test_module="test_names"):
    """Allow unittest selection and a literal fixture cd, never shell programs."""
    if not isinstance(command, str):
        return False
    # Reject substitutions, expansions, redirections, pipelines and newlines
    # before tokenization; shlex alone is not a shell security boundary.
    if any(c in command for c in "$`<>|;\\\n\r"):
        return False
    try:
        words = shlex.split(command)
    except ValueError:
        return False
    if words[:1] == ["cd"]:
        if len(words) < 4 or words[2] != "&&":
            return False
        target = Path(words[1])
        if not target.is_absolute():
            target = workspace / target
        if target.resolve() != workspace.resolve():
            return False
        words = words[3:]
    if len(words) < 3 or words[0] not in {"python", "python3"} or words[1:3] != ["-m", "unittest"]:
        return False
    return all(w == "-v" or re.fullmatch(re.escape(test_module) + r"(?:\.[A-Za-z_][A-Za-z_0-9]*)*", w)
               for w in words[3:])


class Terminal:
    def __init__(self, binary, case_dir, env, coding=False, timeout=240, weather=False, case_policy=None, resource_limits=None):
        self.case_dir = Path(case_dir).resolve()
        self.workspace = self.case_dir / "workspace"
        self.events = []
        self.event_bytes = 0
        self.pending = b""
        self.coding = coding
        self.weather = weather
        self.policy = case_policy or {}
        self.meter = Meter(resource_limits or profile())
        self.decisions = []
        self.deadline = time.monotonic() + timeout
        self.started = time.monotonic()
        self.first_start = None
        self.last_settlement = None
        self.first_text = None
        self.log = self.case_dir / "events.jsonl"
        self.receipts = self.log.open("xb")
        self.pipe = self.case_dir / ".event-pipe"
        os.mkfifo(self.pipe, 0o600)
        self.pipe_fd = os.open(self.pipe, os.O_RDONLY | os.O_NONBLOCK)
        self.raw = (self.case_dir / "terminal.ansi").open("wb")
        executable = self.case_dir / "agent-harness"
        shutil.copy2(binary, executable)
        master, slave = pty.openpty()
        self.master = master
        fcntl.ioctl(slave, termios.TIOCSWINSZ, struct.pack("HHHH", 40, 120, 0, 0))
        env = {**env, "TERM": "xterm-256color", "PATH": "/usr/bin:/bin",
               "AGENT_HARNESS_EVENT_PIPE": str(self.pipe)}
        try:
            self.proc = subprocess.Popen(command_prefix(self.meter.limits) + sandbox(self.case_dir, [executable]),
                                         cwd=self.workspace, env=env, stdin=slave,
                                         stdout=slave, stderr=slave, start_new_session=True)
        except BaseException:
            os.close(master)
            self.raw.close()
            self.receipts.close()
            os.close(self.pipe_fd)
            raise
        finally:
            os.close(slave)

    def send(self, data):
        os.write(self.master, data)

    def pump(self):
        self.meter.observe(self.proc.pid)
        if time.monotonic() >= self.deadline:
            raise TimeoutError("terminal journey exceeded its deadline")
        if select.select([self.master], [], [], 0.05)[0]:
            try:
                data = os.read(self.master, 65536)
            except OSError as error:
                if error.errno != errno.EIO:
                    raise
                data = b""
            self.raw.write(data)
            self.raw.flush()
            # Answer terminal discovery; never interpret model text as controls.
            if b"\x1b[6n" in data:
                self.send(b"\x1b[1;1R")
        fresh = []
        if select.select([self.pipe_fd], [], [], 0)[0]:
            data = os.read(self.pipe_fd, 65536)
            self.event_bytes += len(data)
            if self.event_bytes > 32 * 1024 * 1024:
                raise RuntimeError("event stream exceeds capture budget")
            self.receipts.write(data)
            self.receipts.flush()
            lines = (self.pending + data).split(b"\n")
            self.pending = lines.pop()
            if len(self.pending) > 1024 * 1024 or any(len(line) > 1024 * 1024 for line in lines):
                raise RuntimeError("event record exceeds capture budget")
            for line in lines:
                event = parse_event(line)
                self.events.append(event)
                fresh.append(event)
                if event["type"] == "start" and self.first_start is None:
                    self.first_start = time.monotonic()
                if event["type"] in {"complete", "error"}:
                    self.last_settlement = time.monotonic()
                if event["type"] == "text" and self.first_text is None and event.get("text", "").strip():
                    self.first_text = time.monotonic() - (self.first_start or self.started)
                if event["type"] == "approval":
                    approve = allowed(event["tool"], event.get("input"), self.workspace, self.coding, self.weather,
                                      self.policy.get("allowed_files", ["names.py"])[0],
                                      self.policy.get("test_module", "test_names"))
                    self.decisions.append({"id": event["id"], "tool": event["tool"],
                                           "command": event["command"], "approved": approve})
                    self.send(b"a" if approve else b"r")
                if event["type"] == "question":
                    raise RuntimeError("agent asked an unscripted question; human input required")
        if self.raw.tell() > 32 * 1024 * 1024 or len(self.events) > 50000:
            raise RuntimeError("terminal evidence exceeded its bounded capture budget")
        if self.proc.poll() is not None and not fresh:
            raise RuntimeError(f"app exited before journey completed (exit {self.proc.returncode})")
        return fresh

    def wait(self, predicate):
        while True:
            for event in self.pump():
                if predicate(event):
                    return event

    def settle(self, turn):
        event = self.wait(lambda e: e["type"] in {"complete", "error"} and e["turn"] == turn)
        if event["type"] == "error":
            raise RuntimeError(str(event.get("error", "application error")))
        return event

    def prompt(self, text):
        # Bracketed paste prevents embedded newlines from submitting early.
        self.send(b"\x1b[200~" + text.encode() + b"\x1b[201~")
        self.send(b"\r")

    def journey(self, case):
        self.wait(lambda e: e["type"] == "window")
        self.send(b"c")
        # Let navigation land before entering compose mode.
        for _ in range(4):
            self.pump()
        self.send(b"i")
        self.prompt(case["prompt"])
        if case.get("action"):
            first = self.wait(lambda e: e["type"] in {"text", "complete", "error"})
            if first["type"] != "text":
                raise RuntimeError("turn ended before cancellation could be exercised")
            self.send(b"\x1b")
            self.wait(lambda e: e["type"] == "cancel")
            self.send(b"i")
            self.prompt(case["follow_up"])
            final = self.settle(2)
        else:
            final = self.settle(1)
            if case.get("follow_up"):
                self.prompt(case["follow_up"])
                final = self.settle(2)
        return final

    def close(self):
        if self.proc.poll() is None:
            try:
                os.killpg(self.proc.pid, signal.SIGTERM)
            except ProcessLookupError:
                pass  # The child can exit between poll and kill.
            try:
                self.proc.wait(timeout=5)
            except subprocess.TimeoutExpired:
                try:
                    os.killpg(self.proc.pid, signal.SIGKILL)
                except ProcessLookupError:
                    pass
                self.proc.wait(timeout=5)
        # Preserve any final renderer output without waiting for more events.
        while select.select([self.master], [], [], 0)[0]:
            try:
                data = os.read(self.master, 65536)
                if not data:
                    break
                self.raw.write(data)
            except OSError:
                break
        self.raw.close()
        self.receipts.close()
        os.close(self.pipe_fd)
        self.pipe.unlink(missing_ok=True)
        os.close(self.master)
