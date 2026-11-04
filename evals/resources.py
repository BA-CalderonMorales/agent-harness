"""Linux worker limits and host-side, sampled process-tree accounting.

The external inference server is outside this process tree. RSS sampling is
an early-stop threshold, not a cgroup hard memory limit.
"""
from pathlib import Path
import math
import os
import time


def profile(memory_mib=512, cpu_seconds=60, cpu_count=1):
    if any(not math.isfinite(x) or x <= 0 for x in (memory_mib, cpu_seconds, cpu_count)):
        raise ValueError("resource limits must be finite and positive")
    cpus = sorted(os.sched_getaffinity(0))[:int(cpu_count)]
    if not cpus:
        raise ValueError("at least one CPU is required")
    return {"memory_mib": memory_mib, "cpu_seconds": cpu_seconds, "cpus": cpus,
            "scope": "application and tool processes; excludes inference server",
            "memory_enforcement": "sampled aggregate RSS; not a hard cgroup limit"}


def command_prefix(limits):
    return ["prlimit", f"--cpu={math.ceil(limits['cpu_seconds'])}", "--",
            "taskset", "--cpu-list", ",".join(map(str, limits["cpus"]))]


def tree_sample(pid):
    pending, visited = [pid], set()
    rss = ticks = 0
    while pending:
        current = pending.pop()
        if current in visited:
            continue
        visited.add(current)
        try:
            root = Path(f"/proc/{current}")
            # Names may contain spaces or parentheses; the final ')' ends comm.
            fields = (root / "stat").read_text().rsplit(")", 1)[1].split()
            ticks += int(fields[11]) + int(fields[12])
            for line in (root / "status").read_text().splitlines():
                if line.startswith("VmRSS:"):
                    rss += int(line.split()[1]) * 1024
            pending += [int(n) for n in (root / "task" / str(current) / "children").read_text().split()]
        except (OSError, ValueError, IndexError):
            pass  # A process can finish while the tree is being sampled.
    return {"rss_mib": rss / 1048576, "live_cpu_seconds": ticks / os.sysconf("SC_CLK_TCK"),
            "processes": len(visited)}


class Meter:
    def __init__(self, limits):
        self.limits = limits
        self.started = time.monotonic()
        self.peak_rss_mib = self.peak_processes = self.peak_live_cpu_seconds = 0
        self.exceeded = None

    def observe(self, pid):
        sample = tree_sample(pid)
        self.peak_rss_mib = max(self.peak_rss_mib, sample["rss_mib"])
        self.peak_processes = max(self.peak_processes, sample["processes"])
        self.peak_live_cpu_seconds = max(self.peak_live_cpu_seconds, sample["live_cpu_seconds"])
        if sample["rss_mib"] > self.limits["memory_mib"]:
            self.exceeded = "sampled process-tree memory threshold exceeded"
            raise RuntimeError(self.exceeded)
        if sample["live_cpu_seconds"] > self.limits["cpu_seconds"]:
            self.exceeded = "sampled process-tree CPU threshold exceeded"
            raise RuntimeError(self.exceeded)

    def result(self):
        return {"limits": self.limits, "peak_sampled_rss_mib": self.peak_rss_mib,
                "peak_sampled_processes": self.peak_processes,
                "peak_sampled_live_cpu_seconds": self.peak_live_cpu_seconds,
                "wall_seconds": time.monotonic() - self.started, "limit_exceeded": self.exceeded,
                "accounting_note": "short-lived processes may evade samples; RSS can double-count shared pages"}
