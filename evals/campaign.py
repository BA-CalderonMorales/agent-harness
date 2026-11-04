"""Bounded, paired evaluation of pinned binaries. Never publishes releases.

Automatic decisions apply to explicit objective fixtures. They do not
certify model narration, arbitrary tasks, or the candidate's source code.
"""
import argparse
import hashlib
import json
from pathlib import Path
import shutil
import subprocess
import tempfile
import time

import run as base
from terminal import available
from unattended import run_case
from suite import FAMILIES, scenario
from resources import profile


def objective(report):
    checks = report.get("verification", {}).get("checks", {})
    required = {"tests_unchanged_before_verification", "file_scope_respected"}
    if "hidden_checks_pass" in checks:
        required |= {"hidden_checks_pass", "implementation_changed"}
    else:
        required |= {"independent_tests_pass", "names_changed"}
    return (report.get("status") == "needs_review" and required <= checks.keys()
            and all(checks.values()))


def decide(pairs, expected, min_families=1, resource_ratio=None, same_binary=False):
    if len(pairs) != expected:
        return "retain", "campaign incomplete"
    def cost(report):
        return report.get("total_case_seconds", report.get("journey_seconds"))

    wins = losses = 0
    for pair in pairs:
        candidate = pair["candidate"]
        checks = candidate.get("verification", {}).get("checks", {})
        if not checks.get("file_scope_respected", False) or not checks.get("tests_unchanged_before_verification", False):
            return "reject", "candidate violated fixture integrity"
        before, after = objective(pair["baseline"]), objective(candidate)
        wins += int(after and not before)
        losses += int(before and not after)
    if same_binary:
        return "retain", "identical binary control; outcome differences are sampling variability"
    if losses:
        return "reject", "candidate regressed on a paired task"
    if wins:
        families = {p.get("family", "names") for p in pairs}
        if len(families) < min_families:
            return "retain", "insufficient task-family coverage"
        if resource_ratio is not None:
            old_reports = [p["baseline"] for p in pairs]
            new_reports = [p["candidate"] for p in pairs]
            costs = [cost(r) for r in old_reports + new_reports]
            if any(not isinstance(c, (int, float)) or c <= 0 for c in costs):
                return "retain", "missing resource accounting"
            old_passes, new_passes = sum(map(objective, old_reports)), sum(map(objective, new_reports))
            # Charge failed attempts too: best-of-many successes are not cheap.
            if old_passes and (sum(cost(r) for r in new_reports) / new_passes >
                               sum(cost(r) for r in old_reports) / old_passes * resource_ratio):
                return "retain", "cost per verified completion regressed"
            for pair in pairs:
                old, new = pair["baseline"], pair["candidate"]
                if new.get("resources", {}).get("limit_exceeded"):
                    return "reject", "candidate exceeded resource budget"
                # Compare costs only where both completed the contract.
                if objective(old) and objective(new):
                    for metric in ("journey_seconds",):
                        a, b = old.get(metric), new.get(metric)
                        if a is None or b is None or b > max(a, 0.1) * resource_ratio:
                            return "retain", "candidate resource cost needs more evidence"
        return "canary", "objective improvement in this campaign; broader readiness unestablished"
    return "retain", "no demonstrated objective improvement"


def run_campaign(baseline, candidate, output, cases, repeats, budget, timeout, model, families=None, resource_limits=None):
    cases = list(families) if families else cases
    limits = resource_limits or profile()
    if not cases or set(cases) - (set(FAMILIES) if families else {"coding", "recovery"}):
        raise ValueError("automatic decisions support coding and recovery only")
    import math
    if repeats < 1 or repeats > 6 or not math.isfinite(budget) or not math.isfinite(timeout) or budget <= 0 or timeout <= 0:
        raise ValueError("positive budgets and 1..6 repeats are required")
    output = Path(output).resolve()
    output.mkdir(parents=True, exist_ok=True)
    directory = Path(tempfile.mkdtemp(prefix="campaign-", dir=output))
    start = time.monotonic()
    manifest = {"schema_version": 1, "status": "running", "decision": "retain",
                "scope": "isolated experimental canary only", "pairs": [],
                "cases": cases, "repeats": repeats, "budget_seconds": budget,
                "model": model, "resource_policy": limits, "families": bool(families), "binaries": {}, "started_at": base.timestamp()}

    def save():
        manifest["elapsed_seconds"] = time.monotonic() - start
        base.save_atomic(directory / "campaign.json", manifest)

    save()
    try:
        for role, source in (("baseline", baseline), ("candidate", candidate)):
            dest = directory / role
            shutil.copyfile(source, dest)
            dest.chmod(0o500)
            manifest["binaries"][role] = hashlib.sha256(dest.read_bytes()).hexdigest()
        available()
        base.preflight(model)
        source = base.source_state()
        for index in range(repeats):
            for name in cases:
                case = scenario(name) if families else None
                pair = {"case": name, "repeat": index, "family": name if families else "names"}
                if case:
                    pair["variant_id"] = case["variant_id"]
                # Alternate order to reduce systematic warm-up effects.
                roles = ("baseline", "candidate") if index % 2 == 0 else ("candidate", "baseline")
                for role in roles:
                    remaining = budget - (time.monotonic() - start)
                    if remaining < 1:
                        raise TimeoutError("campaign budget exhausted")
                    case_dir = directory / f"{index}-{name}-{role}"
                    pair[role] = run_case(name, case_dir, directory / role, model,
                                          source, min(timeout, remaining),
                                          case_override=case, resource_limits=limits)
                    # Persist a partial pair too; never hide a charged attempt.
                    manifest["active_pair"] = pair
                    save()
                manifest["pairs"].append(pair)
                manifest.pop("active_pair", None)
                save()
        manifest["decision"], manifest["reason"] = decide(
            manifest["pairs"], repeats * len(cases), min_families=2, resource_ratio=1.25,
            same_binary=manifest["binaries"]["baseline"] == manifest["binaries"]["candidate"])
        manifest["status"] = "finished"
    except KeyboardInterrupt:
        manifest.update(status="interrupted", decision="retain", reason="campaign interrupted")
    except (OSError, RuntimeError, ValueError, TimeoutError, subprocess.TimeoutExpired, base.Blocked) as error:
        manifest.update(status="incomplete", decision="retain", reason=str(error))
    finally:
        manifest["finished_at"] = base.timestamp()
        save()
    return directory, manifest


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--baseline", type=Path, required=True)
    parser.add_argument("--candidate", type=Path, required=True)
    parser.add_argument("--output", type=Path, default=base.RESULTS / "campaigns")
    parser.add_argument("--cases", nargs="+", default=None)
    parser.add_argument("--families", nargs="+", choices=FAMILIES)
    parser.add_argument("--memory-mib", type=float, default=512)
    parser.add_argument("--cpu-seconds", type=float, default=60)
    parser.add_argument("--cpu-count", type=int, default=1)
    parser.add_argument("--repeats", type=int, default=2)
    parser.add_argument("--budget-seconds", type=float, default=600)
    parser.add_argument("--timeout", type=float, default=120)
    parser.add_argument("--model", default=base.DEFAULT_MODEL)
    args = parser.parse_args()
    if args.cases and args.families:
        parser.error("choose cases or families")
    try:
        limits = profile(args.memory_mib, args.cpu_seconds, args.cpu_count)
        directory, manifest = run_campaign(args.baseline, args.candidate, args.output,
                args.cases, args.repeats, args.budget_seconds, args.timeout, args.model,
                families=args.families or (FAMILIES if args.cases is None else None), resource_limits=limits)
    except (ValueError, OSError) as error:
        parser.error(str(error))
    print(f"{manifest['decision']}: {manifest.get('reason', '')}\nEvidence: {directory / 'campaign.json'}")
    return 0 if manifest["status"] == "finished" else 2


if __name__ == "__main__":
    raise SystemExit(main())
