"""Bounded subprocess capture for untrusted fixture verification (Linux)."""
import os
import selectors
import signal
import subprocess
import time

MAX_OUTPUT = 1024 * 1024


def bounded_capture(command, *, cwd=None, env=None, timeout=10, input=None):
    payload = b"" if input is None else input.encode()
    if len(payload) > 8192:
        raise ValueError("verification input exceeds budget")
    process = subprocess.Popen(command, cwd=cwd, env=env, stdin=subprocess.PIPE,
                               stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                               start_new_session=True)
    selector = selectors.DefaultSelector()
    output = {"stdout": bytearray(), "stderr": bytearray()}
    deadline = time.monotonic() + timeout
    try:
        if payload:
            process.stdin.write(payload)
        process.stdin.close()
        selector.register(process.stdout, selectors.EVENT_READ, "stdout")
        selector.register(process.stderr, selectors.EVENT_READ, "stderr")
        while selector.get_map():
            remaining = deadline - time.monotonic()
            if remaining <= 0:
                raise subprocess.TimeoutExpired(command, timeout)
            for key, _ in selector.select(min(remaining, 0.05)):
                data = os.read(key.fileobj.fileno(), 8192)
                if not data:
                    selector.unregister(key.fileobj)
                    continue
                output[key.data].extend(data)
                if sum(map(len, output.values())) > MAX_OUTPUT:
                    raise RuntimeError("verification output exceeds budget")
        code = process.wait(timeout=max(0.01, deadline - time.monotonic()))
        return subprocess.CompletedProcess(command, code,
                    output["stdout"].decode(errors="replace"), output["stderr"].decode(errors="replace"))
    finally:
        # Reap a process group even when the direct child exited but a
        # descendant retained a pipe. No verifier process may outlive its run.
        try:
            os.killpg(process.pid, signal.SIGKILL)
        except ProcessLookupError:
            pass
        process.wait()
        selector.close()
        process.stdout.close()
        process.stderr.close()
        if not process.stdin.closed:
            process.stdin.close()
