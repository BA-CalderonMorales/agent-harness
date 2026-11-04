"""Unattended orchestration; human review remains the final task verdict."""
import datetime as dt
import hashlib
import json
from pathlib import Path
import subprocess
import tempfile
import time

import run as base
from terminal import Terminal, available, sandbox
from grading import isolated_verify
from suite import public_scenario


def review_text(report, events):
    lines = ["# Actual Agent Harness output", "",
             f"Case: {report['case']} | Machine status: {report['status']}", "",
             "Human review is pending. Events and terminal.ansi retain the interaction evidence.", "",
             "## Submitted scenario", "", report["scenario"]["prompt"], ""]
    if report["scenario"].get("follow_up"):
        lines += ["Follow-up: " + report["scenario"]["follow_up"], ""]
    pending = []
    streamed = {}
    for e in events:
        if e["type"] == "text":
            pending.append(e["text"])
            streamed[e["turn"]] = streamed.get(e["turn"], "") + e["text"]
            continue
        if pending:
            lines += ["## Agent (streamed)", "", "".join(pending), ""]
            pending.clear()
        if e["type"] == "submit":
            lines += ["## User", "", e["text"], ""]
        elif e["type"] == "complete":
            if e["text"] != streamed.get(e["turn"]):
                lines += ["## Agent", "", e["text"], ""]
            lines += ["Turn completed.", ""]
        elif e["type"] == "tool_start":
            lines += [f"Tool proposed: `{e['tool']}`", "", "```json",
                      json.dumps(e["input"], indent=2), "```", ""]
        elif e["type"] == "tool_done":
            lines += [f"Tool result: {'success' if e['success'] else 'error'}", "",
                      "```text", e["output"], "```", ""]
        elif e["type"] == "cancel":
            lines += ["Turn cancelled through Esc.", ""]
        elif e["type"] == "error":
            lines += ["Application error: " + e["error"], ""]
    if pending:
        lines += ["## Agent (partial stream)", "", "".join(pending), ""]
    if report.get("automation_error"):
        lines += ["Driver outcome: " + report["automation_error"], ""]
    lines += ["## Independent verification", "", "```json",
              json.dumps(report.get("verification", {}), indent=2), "```", ""]
    return "\n".join(lines)


def run_case(name, directory, binary, model, source, timeout, case_override=None, resource_limits=None):
    directory.mkdir()
    case = case_override or base.resolved_case(base.CASES[name])
    workspace = directory / "workspace"
    before = base.prepare(workspace, case)
    if case.get("fixture") == "generated":
        for filename, content in case["files"].items():
            (workspace / filename).write_text(content)
        before = base.snapshot(workspace)
    subprocess.run(["git", "init", "-q", str(workspace)], check=True)
    for folder in ("home", "config", "data", "sessions"):
        (directory / folder).mkdir()
    (directory / "config/settings.json").write_text(json.dumps(base.make_profile(model)))
    report = {"schema_version": 1, "case": name, "started_at": base.timestamp(),
              "profile": base.make_profile(model), "scenario": public_scenario(case), "family": case.get("family", "names"),
              "source_revision": source["revision"], "source_dirty": source["dirty"],
              "status": "running", "initial_files": before,
              "review": {"task_completion": "unreviewed", "honesty": "unreviewed",
                         "interaction": "unreviewed"}, "automation": "real-tui-pty",
              "first_visible_text_seconds": None, "model_task_seconds": None}
    base.save(directory, report)
    terminal = None
    start = time.monotonic()
    try:
        terminal = Terminal(binary, directory, base.make_env(directory, model),
                            coding=case.get("fixture") in {"names", "generated"}, timeout=timeout, weather=name == "weather",
                            case_policy=case, resource_limits=resource_limits)
        final = terminal.journey(case)
        report["final_response"] = final["text"]
        report["status"] = "needs_review"
    except KeyboardInterrupt:
        report["status"] = "interrupted"
        raise
    except (OSError, RuntimeError, TimeoutError, ValueError) as error:
        report.update(status="fail", automation_error=str(error))
    finally:
        if terminal:
            terminal.close()
            report["driver_shutdown_exit_code"] = terminal.proc.returncode
            # Observation precedes painting; do not claim rendered latency.
            report["first_text_event_seconds"] = terminal.first_text
            if terminal.first_start is not None and terminal.last_settlement is not None:
                report["model_task_seconds"] = terminal.last_settlement - terminal.first_start
            report["approval_decisions"] = terminal.decisions
            report["resources"] = terminal.meter.result()
        events = terminal.events if terminal else []
        report["journey_seconds"] = time.monotonic() - start
        report["finished_at"] = base.timestamp()
        # Model-written Python is verified inside the same OS boundary.
        grading_started = time.monotonic()
        try:
            report["verification"] = isolated_verify(base, workspace, case, before, resource_limits=resource_limits)
            checks = report["verification"]["checks"]
            if name == "greeting":
                checks["no_tools"] = not any(e["type"] == "tool_start" for e in events)
            if name == "cancel":
                checks["cancel_then_continue"] = (any(e["type"] == "cancel" for e in events)
                    and report.get("final_response", "").strip() == "READY_TO_CONTINUE")
            if not all(checks.values()) and report["status"] != "interrupted":
                report["status"] = "fail"
        except (OSError, ValueError, RuntimeError, subprocess.TimeoutExpired) as error:
            report["verification"] = {"checks": {"verification_completed": False}, "error": str(error)}
            if report["status"] != "interrupted":
                report["status"] = "fail"
        report["grading_seconds"] = time.monotonic() - grading_started
        report["total_case_seconds"] = time.monotonic() - start
        base.save(directory, report)
        (directory / "review.md").write_text(review_text(report, events))
    return report


def run_batch(selection, model, prepare_only=False, timeout=240):
    if prepare_only:
        return base.run_batch(selection, model, True)
    base.RESULTS.mkdir(parents=True, exist_ok=True)
    prefix = dt.datetime.now(dt.timezone.utc).strftime("%Y%m%dT%H%M%SZ") + "-unattended-"
    directory = Path(tempfile.mkdtemp(prefix=prefix, dir=base.RESULTS))
    source = base.source_state()
    summary = {"schema_version": 1, "batch": directory.name, "evidence": str(directory),
               "started_at": base.timestamp(), "model": model, "prepare_only": False,
               "source_revision": source["revision"], "source_dirty": source["dirty"],
               "automation": "real-tui-pty", "cases": [], "status": "finished"}
    try:
        available()
        base.preflight(model)
        binary = base.build_app()
        summary["binary_sha256"] = hashlib.sha256(binary.read_bytes()).hexdigest()
    except KeyboardInterrupt:
        summary["status"] = "interrupted"
        summary["cases"] = [{"case": n, "status": "not_run", "report": None} for n in selection]
    except (RuntimeError, base.Blocked, OSError, subprocess.TimeoutExpired) as error:
        summary["status"] = "blocked"
        summary["cases"] = [base.blocked_case(directory, n, str(error), model, source) for n in selection]
    else:
        for i, name in enumerate(selection):
            try:
                report = run_case(name, directory / name, binary, model, source, timeout)
            except KeyboardInterrupt:
                summary["status"] = "interrupted"
                summary["cases"].append({"case": name, "status": "interrupted", "report": f"{name}/report.json"})
                summary["cases"].extend({"case": n, "status": "not_run", "report": None} for n in selection[i+1:])
                break
            summary["cases"].append({"case": name, "status": report["status"],
                                     "report": f"{name}/report.json", "checks": report["verification"]["checks"]})
    summary["counts"] = {s: sum(c["status"] == s for c in summary["cases"]) for s in base.STATUSES}
    summary["finished_at"] = base.timestamp()
    base.save(directory, summary, "summary.json")
    return summary
