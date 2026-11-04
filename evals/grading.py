"""Host-owned verification using a fresh copy, outside the worker's mounts."""
from pathlib import Path
import os
import tempfile
import json
import subprocess
from resources import profile, command_prefix
from capture import bounded_capture

from terminal import sandbox


def isolated_verify(base, workspace, case, before, resource_limits=None):
    limits = resource_limits or profile()
    # Reject indirection before host-side snapshot/read operations.
    for root, dirs, files in os.walk(workspace, followlinks=False):
        for name in dirs + files:
            path = Path(root) / name
            import stat
            if path.is_symlink() or not (stat.S_ISREG(path.stat().st_mode) or stat.S_ISDIR(path.stat().st_mode)):
                return {"checks": {"no_symlinks": False}, "changed_files": [], "test_run": None}
    with tempfile.TemporaryDirectory(prefix="ah-private-grader-") as directory:
        target = Path(directory) / "workspace"
        target.mkdir()
        if case.get("fixture") == "generated":
            return verify_generated(base, workspace, case, before, target, directory, limits)
        if case.get("fixture") == "names":
            # O_NOFOLLOW closes the final-component symlink race as well.
            for name in ("names.py", "test_names.py"):
                source = workspace / name
                if not source.exists():
                    continue
                fd = os.open(source, os.O_RDONLY | getattr(os, "O_NOFOLLOW", 0) | getattr(os, "O_NONBLOCK", 0))
                with os.fdopen(fd, "rb") as stream:
                    import stat
                    if not stat.S_ISREG(os.fstat(stream.fileno()).st_mode):
                        raise ValueError("grader accepts regular source files only")
                    data = stream.read(1024 * 1024 + 1)
                if len(data) > 1024 * 1024:
                    raise ValueError("fixture source exceeds grading budget")
                (target / name).write_bytes(data)
        # Original workspace is inspected by the trusted host. The subprocess
        # is chdir'ed by bwrap into the fresh grading workspace, with no worker
        # home, settings, reports or session files exposed.
        prefix = command_prefix(limits) + ["prlimit", f"--as={int(limits['memory_mib'] * 1048576)}", "--"]
        return base.verify(workspace, case, before, command_prefix=prefix + sandbox(directory, [], network=False))


def verify_generated(base, workspace, case, before, target, directory, limits):
    import stat
    intact = True
    for name, original in case["files"].items():
        fd = os.open(workspace / name, os.O_RDONLY | getattr(os, "O_NOFOLLOW", 0) | getattr(os, "O_NONBLOCK", 0))
        with os.fdopen(fd, "rb") as stream:
            if not stat.S_ISREG(os.fstat(stream.fileno()).st_mode):
                raise ValueError("grader accepts regular files only")
            data = stream.read(1024 * 1024 + 1)
        if len(data) > 1024 * 1024:
            raise ValueError("source exceeds grading budget")
        (target / name).write_bytes(data)
        if name not in case["allowed_files"]:
            intact = intact and data == original.encode()
    after = base.snapshot(workspace)
    changed = sorted(n for n in before.keys() | after.keys() if before.get(n) != after.get(n))
    checks = {"tests_unchanged_before_verification": intact,
              "file_scope_respected": set(changed) <= set(case["allowed_files"]),
              "implementation_changed": bool(set(changed) & set(case["allowed_files"]))}
    # Expected answers are compared by the host; candidate code never receives
    # the expected values or runs inside the host evaluator's process.
    script = (f"from {case['module']} import {case['function']} as f\n"
              "import sys,json\nvalues=json.load(sys.stdin)\n"
              "print(json.dumps([f(v) for v in values]))\n")
    prefix = command_prefix(limits) + ["prlimit", f"--as={int(limits['memory_mib'] * 1048576)}", "--"]
    result = bounded_capture(prefix + sandbox(directory, ["/usr/bin/python3", "-c", script], network=False),
                            input=json.dumps(case["private_inputs"]), timeout=10, env=base.sanitized_env())
    try:
        actual = json.loads(result.stdout)
    except (ValueError, TypeError):
        actual = None
    checks["hidden_checks_pass"] = result.returncode == 0 and actual == case["private_expected"]
    return {"checks": checks, "changed_files": changed,
            "private_check_count": len(case["private_inputs"]),
            "test_run": {"exit_code": result.returncode, "stderr": result.stderr[:4000]}}
