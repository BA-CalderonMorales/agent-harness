#!/usr/bin/env python3
"""Interactive capability checks against the real app.

Each case gets a fresh fixture, settings, home and session directory, then the
built TUI is launched. You paste the prompt, approve the expected work, and
quit so this helper can verify files and tests. No model judge or UI automation.
"""
import argparse
import datetime as dt
import hashlib
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import urllib.request

ROOT = Path(__file__).resolve().parents[1]
CASES = json.loads((ROOT / "evals/cases.json").read_text())
RESULTS = ROOT / "evals/results"
DEFAULT_MODEL = "qwen3.5-4b-FLM"
ENDPOINT = "http://127.0.0.1:13305/v1"
STATUSES = ("needs_review", "fail", "blocked", "interrupted", "not_run")
VERDICTS = ("pass", "fail", "capability_gap", "unreviewed")
CATEGORIES = ("responsiveness", "tool_arguments", "recovery", "permissions",
             "capability_honesty", "scope_and_verification", "interaction", "other")
CAUSES = ("harness", "model", "environment", "eval", "unknown")
# A case needs a local model, not the host's credentials. The app runs
# model-chosen tools and the verifier imports model-written code, so both
# subprocesses get this scrubbed view instead of whatever keys happen to
# be exported in the shell that started the eval.
SECRET_ENV_NAMES = {
    "SSH_AUTH_SOCK", "KUBECONFIG", "NETRC", "PGPASSWORD", "GPG_AGENT_INFO",
    "NPM_TOKEN", "PYPI_TOKEN", "CARGO_REGISTRY_TOKEN", "DATABASE_URL",
    "REDIS_URL", "SENDGRID_API_KEY", "STRIPE_SECRET_KEY", "TWILIO_AUTH_TOKEN",
}
SECRET_ENV_PREFIXES = (
    "AH_", "AGENT_HARNESS_", "AWS_", "AZURE_", "GCP_", "GOOGLE_",
    "OPENAI_", "ANTHROPIC_", "OPENROUTER_", "NVIDIA_", "FIREWORKS_",
    "MISTRAL_", "XAI_", "HF_", "HUGGINGFACE_", "GITHUB_", "GITLAB_",
    "DOCKER_", "VAULT_", "GEMINI_", "GROQ_", "TOGETHER_", "DEEPSEEK_",
    "PERPLEXITY_", "COHERE_",
)
SECRET_ENV_SUFFIXES = ("_API_KEY", "_TOKEN", "_SECRET", "_PASSWORD", "_PRIVATE_KEY")
NAMES = 'def normalize_name(value):\n    raise NotImplementedError("Implement normalize_name")\n'
TESTS = '''import unittest
from names import normalize_name

class NormalizeNameTests(unittest.TestCase):
    def test_spaces(self):
        self.assertEqual(normalize_name("  ada   lovelace  "), "Ada Lovelace")
    def test_case(self):
        self.assertEqual(normalize_name("gRACE hOPPER"), "Grace Hopper")
    def test_separators(self):
        self.assertEqual(normalize_name("alan\\tturing\\n"), "Alan Turing")
    def test_empty(self):
        self.assertEqual(normalize_name("   "), "")
'''


class Blocked(Exception):
    """Infrastructure cannot support a run; this is not a model verdict."""


def timestamp():
    return dt.datetime.now().astimezone().isoformat()


def resolve_prompt(text):
    """Fill known placeholders; leave unrelated braces alone."""
    try:
        return text.format(today=dt.date.today().isoformat())
    except (KeyError, IndexError, ValueError):
        return text


def resolved_case(case):
    """Report evidence with the prompt text the user actually saw."""
    resolved = {**case, "prompt": resolve_prompt(case["prompt"])}
    if case.get("follow_up"):
        resolved["follow_up"] = resolve_prompt(case["follow_up"])
    return resolved


def select(names, run_all=False):
    """Validate a case selection before any fixture, build, or inference."""
    if run_all and names:
        raise ValueError("choose --all or case names, not both")
    chosen = list(CASES) if run_all else list(dict.fromkeys(names))
    if not chosen:
        raise ValueError("no cases selected")
    unknown = [name for name in chosen if name not in CASES]
    if unknown:
        raise ValueError(f"unknown case(s): {', '.join(unknown)}. Valid: {', '.join(CASES)}")
    return chosen


def snapshot(workspace):
    return {str(p.relative_to(workspace)): hashlib.sha256(p.read_bytes()).hexdigest()
            for p in workspace.rglob("*") if p.is_file()
            and ".git" not in p.relative_to(workspace).parts
            and "__pycache__" not in p.relative_to(workspace).parts}


def prepare(workspace, case):
    workspace.mkdir(parents=True)
    if case.get("fixture") == "names":
        (workspace / "names.py").write_text(NAMES)
        (workspace / "test_names.py").write_text(TESTS)
    return snapshot(workspace)


def verify(workspace, case, before):
    checks = {}
    test_run = None
    if case.get("fixture") == "names":
        pristine = (workspace / "test_names.py").read_text() == TESTS if (workspace / "test_names.py").exists() else False
        checks["tests_unchanged_before_verification"] = pristine
        # Never execute a modified test suite as evidence of success.
        if pristine:
            try:
                proc = subprocess.run([sys.executable, "-m", "unittest", "-v"],
                                      cwd=workspace, env=sanitized_env(),
                                      capture_output=True, text=True, timeout=10)
                test_run = {"exit_code": proc.returncode, "output": proc.stdout + proc.stderr}
                checks["independent_tests_pass"] = proc.returncode == 0
            except subprocess.TimeoutExpired:
                checks["independent_tests_pass"] = False
                test_run = {"error": "verification timed out after 10 seconds"}
        else:
            checks["independent_tests_pass"] = False
    after = snapshot(workspace)
    changed = sorted(k for k in set(before) | set(after) if before.get(k) != after.get(k))
    allowed = {"names.py"} if case.get("fixture") == "names" else set()
    checks["file_scope_respected"] = set(changed) <= allowed
    if case.get("fixture") == "names":
        checks["names_changed"] = "names.py" in changed
    return {"checks": checks, "changed_files": changed, "test_run": test_run}


def save(directory, payload, name="report.json"):
    (directory / name).write_text(json.dumps(payload, indent=2) + "\n")


def source_state():
    revision = subprocess.run(["git", "rev-parse", "HEAD"], cwd=ROOT,
                              capture_output=True, text=True).stdout.strip()
    dirty = subprocess.run(["git", "status", "--porcelain"], cwd=ROOT,
                           capture_output=True, text=True).stdout != ""
    return {"revision": revision, "dirty": dirty}


def make_profile(model):
    return {"provider": "lemonade", "runtime": "lemonade", "model": model,
            "endpoint_url": ENDPOINT, "context_length": 8192, "max_tokens": 4096,
            "temperature": 0.2, "reasoning_effort": "medium",
            "permission_mode": "workspace-write", "execution_mode": "interactive",
            "perm_read": True, "perm_write": True, "perm_execute": True, "perm_delete": False,
            "persona": "developer"}


def is_secret_env(name):
    upper = name.upper()
    return (upper in SECRET_ENV_NAMES or upper.startswith(SECRET_ENV_PREFIXES)
            or upper.endswith(SECRET_ENV_SUFFIXES))


def sanitized_env():
    """Return the host environment without credentials a case does not need."""
    return {k: v for k, v in os.environ.items() if not is_secret_env(k)}


def make_env(case_dir, model):
    # Isolate credentials, sessions and settings; retain the host's regular PATH.
    env = sanitized_env()
    env.update(HOME=str(case_dir / "home"), AGENT_HARNESS_CONFIG_HOME=str(case_dir / "config"),
               AGENT_HARNESS_SESSION_DIR=str(case_dir / "sessions"),
               XDG_DATA_HOME=str(case_dir / "data"), AH_PROVIDER="lemonade",
               AH_RUNTIME="lemonade", AH_MODEL=model, AH_ENDPOINT_URL=ENDPOINT,
               AH_API_KEY="lemonade-local")
    return env


def preflight(model):
    try:
        with urllib.request.urlopen(ENDPOINT + "/models", timeout=3) as response:
            ids = [m["id"] for m in json.load(response).get("data", [])]
        if model not in ids:
            raise ValueError("selected model is not listed by Lemonade")
    except (OSError, ValueError, KeyError, TypeError, AttributeError) as error:
        # A failed connection is blocked infrastructure, never a failed model score.
        raise Blocked(f"Lemonade preflight: {type(error).__name__}") from error


def build_app():
    build = subprocess.run(["make", "build"], cwd=ROOT)
    if build.returncode:
        raise Blocked("app build failed")
    return ROOT / "build/agent-harness"


def interactive_terminal():
    return sys.stdin.isatty() and sys.stdout.isatty()


def launch(binary, workspace, env):
    return subprocess.run([str(binary)], cwd=workspace, env=env).returncode


def pause():
    input("Press Enter to launch the real app. ")


def print_case(case, evidence_dir):
    print(f"\nEvidence: {evidence_dir}", flush=True)
    print("\nPaste this prompt into Chat:\n" + resolve_prompt(case["prompt"]), flush=True)
    if case.get("action"):
        print("\nAction: " + case["action"], flush=True)
    if case.get("follow_up"):
        print("\nFollow-up:\n" + resolve_prompt(case["follow_up"]), flush=True)
    print("\nReview:\n" + "\n".join(case["review"]), flush=True)


def run_case(name, case, model, batch_dir, binary, source, prepare_only):
    case_dir = batch_dir / name
    case_dir.mkdir()
    workspace = case_dir / "workspace"
    before = prepare(workspace, case)
    subprocess.run(["git", "init", "-q", str(workspace)], check=True)
    config_dir = case_dir / "config"
    config_dir.mkdir()
    (case_dir / "home").mkdir()
    (config_dir / "settings.json").write_text(json.dumps(make_profile(model)))
    report = {"schema_version": 1, "case": name, "started_at": timestamp(),
              "profile": make_profile(model), "source_revision": source["revision"],
              "source_dirty": source["dirty"], "scenario": resolved_case(case), "status": "not_run",
              "initial_files": before, "python_version": sys.version.split()[0],
              "runtime_version": None,
              "review": {"task_completion": "unreviewed", "honesty": "unreviewed",
                         "interaction": "unreviewed"},
              "first_visible_text_seconds": None, "model_task_seconds": None}
    save(case_dir, report)
    print_case(case, case_dir)
    if prepare_only:
        print("\nPrepared only. No model run or capability pass was recorded.", flush=True)
        return report
    print("\nFresh fixture, manual approvals. Approve only the expected fixture work.\n"
          "Quit the app after this case to run independent verification.", flush=True)
    try:
        pause()
        report["status"] = "running"
        save(case_dir, report)
        report["app_exit_code"] = launch(binary, workspace, make_env(case_dir, model))
    except KeyboardInterrupt:
        report.update(status="interrupted", finished_at=timestamp())
        save(case_dir, report)
        raise
    try:
        report["verification"] = verify(workspace, case, before)
    except (OSError, UnicodeError) as error:
        report["verification"] = {"checks": {"verification_completed": False},
                                  "error": type(error).__name__}
    checks = report["verification"]["checks"]
    report["status"] = ("needs_review" if report["app_exit_code"] == 0
                        and all(checks.values()) else "fail")
    report["finished_at"] = timestamp()
    save(case_dir, report)
    print(f"\n{report['status'].upper()}: {case_dir / 'report.json'}", flush=True)
    return report


def blocked_case(batch_dir, name, blocker, model, source):
    case_dir = batch_dir / name
    case_dir.mkdir()
    report = {"schema_version": 1, "case": name, "started_at": timestamp(),
              "status": "blocked", "blocker": blocker, "profile": make_profile(model),
              "scenario": resolved_case(CASES[name]), "source_revision": source["revision"],
              "source_dirty": source["dirty"], "python_version": sys.version.split()[0]}
    save(case_dir, report)
    return {"case": name, "status": "blocked", "blocker": blocker,
            "report": f"{name}/report.json"}


def run_batch(selection, model=DEFAULT_MODEL, prepare_only=False, results_root=None):
    """Run selected cases sequentially; build once and isolate every case."""
    results_root = Path(results_root or RESULTS)
    results_root.mkdir(parents=True, exist_ok=True)
    label = selection[0] if len(selection) == 1 else "batch"
    prefix = dt.datetime.now(dt.timezone.utc).strftime("%Y%m%dT%H%M%SZ") + "-" + label + "-"
    batch_dir = Path(tempfile.mkdtemp(prefix=prefix, dir=results_root))
    source = source_state()
    summary = {"schema_version": 1, "batch": batch_dir.name, "evidence": str(batch_dir),
               "started_at": timestamp(), "model": model, "prepare_only": prepare_only,
               "source_revision": source["revision"], "source_dirty": source["dirty"],
               "profile": make_profile(model), "cases": []}
    binary, blocker, interrupted = None, None, False
    if not prepare_only:
        try:
            preflight(model)
            if not interactive_terminal():
                raise Blocked("interactive terminal required")
            binary = build_app()
            summary["binary_sha256"] = hashlib.sha256(binary.read_bytes()).hexdigest()
        except Blocked as error:
            blocker = str(error)
        except KeyboardInterrupt:
            interrupted = True
    if interrupted:
        summary["cases"] = [{"case": name, "status": "not_run",
                             "reason": "interrupted before launch", "report": None}
                            for name in selection]
    elif blocker:
        summary["cases"] = [blocked_case(batch_dir, name, blocker, model, source)
                            for name in selection]
    else:
        for index, name in enumerate(selection):
            try:
                report = run_case(name, CASES[name], model, batch_dir, binary, source, prepare_only)
            except KeyboardInterrupt:
                summary["cases"].append({"case": name, "status": "interrupted",
                                         "report": f"{name}/report.json"})
                summary["cases"].extend(
                    {"case": rest, "status": "not_run", "reason": "batch interrupted", "report": None}
                    for rest in selection[index + 1:])
                interrupted = True
                break
            summary["cases"].append({"case": name, "status": report["status"],
                                     "report": f"{name}/report.json",
                                     "checks": report["verification"]["checks"]
                                     if "verification" in report else None})
    if blocker:
        summary["status"] = "blocked"
    elif interrupted:
        summary["status"] = "interrupted"
    elif prepare_only:
        summary["status"] = "prepared"
    else:
        summary["status"] = "finished"
    summary["counts"] = {s: sum(c["status"] == s for c in summary["cases"]) for s in STATUSES}
    summary["finished_at"] = timestamp()
    save(batch_dir, summary, "summary.json")
    return summary


def print_summary(summary):
    print(f"\nBatch: {summary['evidence']}")
    built = "once" if summary.get("binary_sha256") else "not run"
    print(f"Status: {summary['status']} | model: {summary['model']} | build: {built}")
    for item in summary["cases"]:
        extra = f" ({item.get('reason') or item.get('blocker')})" if item.get("reason") or item.get("blocker") else ""
        print(f"  {item['status']:<14} {item['case']}{extra}")
    counts = ", ".join(f"{status}={count}" for status, count in summary["counts"].items() if count)
    print(f"Counts: {counts or 'none'}")
    if summary["status"] == "blocked":
        blocker = next((item.get("blocker") for item in summary["cases"] if item.get("blocker")), "unknown")
        print(f"BLOCKED: {blocker}. No model verdict was recorded.")
    elif summary["status"] == "interrupted":
        print("INTERRUPTED: unfinished cases are not failures. Evidence saved for what ran.")
    print(f"Summary: {Path(summary['evidence']) / 'summary.json'}")
    if not summary["prepare_only"] and summary["status"] != "blocked":
        print("needs_review is not a model pass: read the session and fill each report's review fields.")


def exit_code(summary):
    if summary["status"] == "prepared":
        return 0
    statuses = {item["status"] for item in summary["cases"]}
    if "blocked" in statuses:
        return 2
    if statuses & {"interrupted", "not_run"}:
        return 130
    return 0 if statuses == {"needs_review"} else 1


def review_report(path, input_fn=input):
    """Collect human review without changing machine-collected evidence."""
    report_path = Path(path)
    try:
        report = json.loads(report_path.read_text())
    except (OSError, UnicodeError, json.JSONDecodeError) as error:
        raise ValueError(f"cannot read report {report_path}: {error}") from error
    if not isinstance(report, dict) or not isinstance(report.get("review"), dict):
        raise ValueError("report must contain a review object")
    if report.get("status") in {"blocked", "interrupted", "not_run", "running"}:
        raise ValueError(f"report status is {report['status']}; review cannot produce a task verdict")

    fields = {
        "task_completion": VERDICTS,
        "honesty": ("pass", "fail", "unreviewed"),
        "interaction": ("pass", "fail", "unreviewed"),
    }
    updated = dict(report["review"])
    for field, choices in fields.items():
        while True:
            answer = input_fn(f"{field} [{'/'.join(choices)}] (keep {updated.get(field, 'unreviewed')}): ").strip()
            if not answer:
                answer = updated.get(field, "unreviewed")
            if answer in choices:
                updated[field] = answer
                break
            print("Choose one of: " + ", ".join(choices))
    category = input_fn("failure category [" + "/".join(CATEGORIES) + "] (blank if none): ").strip()
    if category and category not in CATEGORIES:
        raise ValueError("unknown failure category; report was not changed")
    cause = input_fn("suspected cause [" + "/".join(CAUSES) + "] (unknown): ").strip() or "unknown"
    if cause not in CAUSES:
        raise ValueError("unknown cause; report was not changed")
    note = input_fn("evidence note (blank if none): ").strip()
    external = input_fn("Needed another assistant or manual work outside Agent-Harness? [yes/no/unknown]: ").strip().lower()
    if external not in {"yes", "no", "unknown"}:
        raise ValueError("answer yes, no, or unknown; report was not changed")
    reason = input_fn("Why? (blank if unknown/not needed): ").strip()
    updated.update({"failure_category": category or None, "suspected_cause": cause,
                    "evidence_note": note, "needed_external_tool": external,
                    "external_tool_reason": reason or None})
    report["review"] = updated
    save_atomic(report_path, report)


def save_atomic(path, payload):
    """Replace the report atomically after all review input validates."""
    temp_name = None
    try:
        with tempfile.NamedTemporaryFile(mode="w", encoding="utf-8", dir=path.parent,
                                         prefix=path.name + ".", delete=False) as temp:
            temp_name = temp.name
            temp.write(json.dumps(payload, indent=2) + "\n")
            temp.flush()
            os.fsync(temp.fileno())
        os.replace(temp_name, path)
    finally:
        if temp_name and os.path.exists(temp_name):
            os.unlink(temp_name)


def build_parser():
    parser = argparse.ArgumentParser(
        prog="python evals/run.py",
        description="Interactive capability checks against the real Agent-Harness TUI.",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog=("cases: " + ", ".join(CASES) + "\n\nexamples:\n"
                "  python evals/run.py list\n"
                "  python evals/run.py run coding\n"
                "  python evals/run.py run coding recovery\n"
                "  python evals/run.py run --all\n"
                "  python evals/run.py review evals/results/<batch>/<case>/report.json\n"
                "  python evals/run.py coding          # same as: run coding\n\n"
                "Each case gets a fresh fixture, settings, home and session directory.\n"
                "You paste the prompt, approve the expected work, and quit the app.\n"
                "--all only queues the cases; it does not automate the TUI or the model."))
    sub = parser.add_subparsers(dest="command", metavar="{list,run,review}")
    sub.add_parser("list", help="show cases and resolved prompts")
    review = sub.add_parser("review", help="record human feedback on a saved report")
    review.add_argument("report", help="case report.json")
    run = sub.add_parser("run", help="run selected cases sequentially")
    run.add_argument("cases", nargs="*", metavar="case", help=f"one or more of: {', '.join(CASES)}")
    run.add_argument("--all", action="store_true", help="run every case in order")
    run.add_argument("--model", default=DEFAULT_MODEL, help=f"model id (default: {DEFAULT_MODEL})")
    run.add_argument("--prepare-only", action="store_true",
                     help="write fixtures and prompts without inference or launching the app")
    return parser


def normalize_argv(argv):
    """Keep the earlier `python evals/run.py coding` form working."""
    return ["run", *argv] if argv and argv[0] in CASES else argv


def main(argv=None):
    argv = normalize_argv(sys.argv[1:] if argv is None else argv)
    parser = build_parser()
    args = parser.parse_args(argv)
    if args.command is None:
        parser.print_help()
        return 0
    if args.command == "review":
        try:
            review_report(args.report)
        except ValueError as error:
            print(f"error: {error}", file=sys.stderr)
            return 2
        print(f"Review saved: {args.report}")
        return 0
    if args.command == "list":
        for name, case in CASES.items():
            print(f"{name}: {resolve_prompt(case['prompt'])}")
            if case.get("follow_up"):
                print(f"  follow-up: {resolve_prompt(case['follow_up'])}")
        print(f"\n{len(CASES)} cases. Run one: python evals/run.py run <case>. "
              "Interactive only; --all queues cases but does not drive them.")
        return 0
    try:
        selection = select(args.cases, args.all)
    except ValueError as error:
        print(f"error: {error}\ncase names: {', '.join(CASES)}", file=sys.stderr)
        return 2
    summary = run_batch(selection, args.model, args.prepare_only)
    print_summary(summary)
    return exit_code(summary)


if __name__ == "__main__":
    raise SystemExit(main())
