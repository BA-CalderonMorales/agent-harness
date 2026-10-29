"""CLI and batch behavior. These tests never call a model or build the app."""
import contextlib
import datetime as dt
import importlib.util
import io
import json
from pathlib import Path
import tempfile
import unittest
from unittest import mock

spec = importlib.util.spec_from_file_location("eval_cli", Path(__file__).with_name("run.py"))
runner = importlib.util.module_from_spec(spec)
spec.loader.exec_module(runner)

SOLUTION = 'def normalize_name(value):\n    return " ".join(word.capitalize() for word in value.split())\n'


class FakeInfra:
    """Stand-in for the model server, build and interactive app."""

    def __init__(self, test_case, exit_codes=None, interrupt_on=None):
        self.temp = tempfile.TemporaryDirectory()
        test_case.addCleanup(self.temp.cleanup)
        self.binary = Path(self.temp.name) / "agent-harness"
        self.binary.write_bytes(b"fake binary")
        self.results = Path(self.temp.name) / "results"
        self.exit_codes = list(exit_codes or [])
        self.interrupt_on = interrupt_on
        self.built = 0
        self.launched = []

    def build(self):
        self.built += 1
        return self.binary

    def launch(self, binary, workspace, env):
        self.launched.append((workspace, env))
        if len(self.launched) == self.interrupt_on:
            raise KeyboardInterrupt
        if (workspace / "test_names.py").exists():
            (workspace / "names.py").write_text(SOLUTION)
        index = len(self.launched) - 1
        return self.exit_codes[index] if index < len(self.exit_codes) else 0


class EvalCliTests(unittest.TestCase):
    def invoke(self, infra, *argv, preflight=None):
        with contextlib.ExitStack() as stack:
            stack.enter_context(mock.patch.object(runner, "RESULTS", infra.results))
            stack.enter_context(mock.patch.object(runner, "interactive_terminal", lambda: True))
            stack.enter_context(mock.patch.object(runner, "preflight", preflight or (lambda model: None)))
            stack.enter_context(mock.patch.object(runner, "build_app", infra.build))
            stack.enter_context(mock.patch.object(runner, "launch", infra.launch))
            stack.enter_context(mock.patch.object(runner, "pause", lambda: None))
            out, err = io.StringIO(), io.StringIO()
            with contextlib.redirect_stdout(out), contextlib.redirect_stderr(err):
                code = runner.main(list(argv))
        return code, out.getvalue(), err.getvalue()

    def batch_dir(self, infra):
        return next(infra.results.iterdir())

    def summary(self, infra):
        return json.loads((self.batch_dir(infra) / "summary.json").read_text())

    def test_no_arguments_prints_help(self):
        code, out, _ = self.invoke(FakeInfra(self))
        self.assertEqual(code, 0)
        self.assertIn("usage: python evals/run.py", out)
        self.assertIn("run --all", out)
        self.assertIn("does not automate", out)

    def test_list_resolves_today_and_shows_follow_ups(self):
        code, out, _ = self.invoke(FakeInfra(self), "list")
        self.assertEqual(code, 0)
        for name in runner.CASES:
            self.assertIn(name, out)
        self.assertIn(dt.date.today().isoformat(), out)
        self.assertNotIn("{today}", out)
        self.assertIn("Try one more time.", out)

    def test_invalid_selection_fails_before_any_work(self):
        infra = FakeInfra(self)
        code, _, err = self.invoke(infra, "run", "nope")
        self.assertEqual(code, 2)
        self.assertIn("unknown case", err)
        self.assertEqual(infra.built, 0)
        self.assertEqual(infra.launched, [])

        code, _, err = self.invoke(infra, "run", "--all", "coding")
        self.assertEqual(code, 2)
        self.assertIn("--all or case names", err)

        code, _, err = self.invoke(infra, "run")
        self.assertEqual(code, 2)
        self.assertIn("no cases selected", err)

    def test_select_dedupes_and_expands_all(self):
        self.assertEqual(runner.select(["coding", "coding"]), ["coding"])
        self.assertEqual(runner.select([], run_all=True), list(runner.CASES))
        with self.assertRaises(ValueError):
            runner.select(["weather", "unknown"])

    def test_all_runs_sequentially_with_one_build_and_isolated_cases(self):
        infra = FakeInfra(self)
        code, out, _ = self.invoke(infra, "run", "--all")
        self.assertEqual(code, 0)
        self.assertEqual(infra.built, 1)
        self.assertEqual(len(infra.launched), len(runner.CASES))
        workspaces = [workspace for workspace, _ in infra.launched]
        self.assertEqual(len({str(path) for path in workspaces}), len(runner.CASES))
        homes = {env["HOME"] for _, env in infra.launched}
        self.assertEqual(len(homes), len(runner.CASES))

        summary = self.summary(infra)
        self.assertEqual(summary["status"], "finished")
        self.assertEqual(summary["counts"]["needs_review"], len(runner.CASES))
        for name in runner.CASES:
            report = json.loads((self.batch_dir(infra) / name / "report.json").read_text())
            self.assertEqual(report["status"], "needs_review")
            self.assertEqual(report["source_revision"], summary["source_revision"])
        self.assertIn("Counts:", out)

    def test_ordinary_failure_does_not_stop_later_cases(self):
        infra = FakeInfra(self, exit_codes=[1])
        code, _, _ = self.invoke(infra, "run", "coding", "greeting")
        self.assertEqual(code, 1)
        self.assertEqual(infra.built, 1)
        self.assertEqual(len(infra.launched), 2)
        statuses = [item["status"] for item in self.summary(infra)["cases"]]
        self.assertEqual(statuses, ["fail", "needs_review"])

    def test_interruption_saves_evidence_and_marks_remaining_not_run(self):
        infra = FakeInfra(self, interrupt_on=2)
        code, out, _ = self.invoke(infra, "run", "coding", "greeting", "weather")
        self.assertEqual(code, 130)
        summary = self.summary(infra)
        self.assertEqual(summary["status"], "interrupted")
        statuses = [item["status"] for item in summary["cases"]]
        self.assertEqual(statuses, ["needs_review", "interrupted", "not_run"])
        interrupted = json.loads((self.batch_dir(infra) / "greeting" / "report.json").read_text())
        self.assertEqual(interrupted["status"], "interrupted")
        self.assertIn("INTERRUPTED", out)
        self.assertEqual(infra.launched[-1][0], self.batch_dir(infra) / "greeting" / "workspace")

    def test_blocked_preflight_records_every_case_without_building(self):
        infra = FakeInfra(self)

        def blocked(model):
            raise runner.Blocked("Lemonade preflight: URLError")

        code, out, _ = self.invoke(infra, "run", "coding", "weather", preflight=blocked)
        self.assertEqual(code, 2)
        self.assertEqual(infra.built, 0)
        self.assertEqual(infra.launched, [])
        summary = self.summary(infra)
        self.assertEqual(summary["status"], "blocked")
        self.assertEqual(summary["counts"]["blocked"], 2)
        self.assertIn("BLOCKED", out)
        for name in ("coding", "weather"):
            report = json.loads((self.batch_dir(infra) / name / "report.json").read_text())
            self.assertEqual(report["status"], "blocked")
            self.assertIn("Lemonade preflight", report["blocker"])

    def test_noninteractive_terminal_is_blocked_before_building(self):
        infra = FakeInfra(self)
        with contextlib.ExitStack() as stack:
            stack.enter_context(mock.patch.object(runner, "RESULTS", infra.results))
            stack.enter_context(mock.patch.object(runner, "preflight", lambda model: None))
            stack.enter_context(mock.patch.object(runner, "interactive_terminal", lambda: False))
            stack.enter_context(mock.patch.object(runner, "build_app", infra.build))
            out = io.StringIO()
            with contextlib.redirect_stdout(out):
                code = runner.main(["run", "coding"])
        self.assertEqual(code, 2)
        self.assertEqual(infra.built, 0)
        self.assertEqual(self.summary(infra)["counts"]["blocked"], 1)
        self.assertIn("build: not run", out.getvalue())
        self.assertIn("interactive terminal required", out.getvalue())

    def test_prepare_only_writes_fixtures_without_building(self):
        infra = FakeInfra(self)
        code, out, _ = self.invoke(infra, "run", "weather", "--prepare-only")
        self.assertEqual(code, 0)
        self.assertEqual(infra.built, 0)
        self.assertEqual(infra.launched, [])
        self.assertIn(dt.date.today().isoformat(), out)
        self.assertNotIn("{today}", out)
        summary = self.summary(infra)
        self.assertEqual(summary["status"], "prepared")
        self.assertEqual(summary["counts"]["not_run"], 1)
        report = json.loads((self.batch_dir(infra) / "weather" / "report.json").read_text())
        self.assertIn(dt.date.today().isoformat(), report["scenario"]["prompt"])
        self.assertNotIn("{today}", report["scenario"]["prompt"])

        code, _, _ = self.invoke(infra, "run", "coding", "--prepare-only")
        workspace = self.batch_dir(infra) / "coding" / "workspace"
        self.assertTrue((workspace / "names.py").exists())
        self.assertTrue((workspace / "test_names.py").exists())
        self.assertEqual(code, 0)

    def test_legacy_single_case_form_still_runs(self):
        infra = FakeInfra(self)
        code, _, _ = self.invoke(infra, "coding")
        self.assertEqual(code, 0)
        self.assertEqual(infra.built, 1)
        self.assertEqual(len(infra.launched), 1)
        self.assertEqual(self.summary(infra)["counts"]["needs_review"], 1)


if __name__ == "__main__":
    unittest.main()

class ReviewTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.path = Path(self.temp.name) / "report.json"
        self.original = {"status": "needs_review", "checks": {"passed": True},
                         "source_revision": "abc", "review": {"task_completion": "unreviewed"}}
        self.path.write_text(json.dumps(self.original))

    def test_review_saves_feedback_and_preserves_evidence(self):
        answers = iter(["pass", "pass", "pass", "", "unknown", "4 tests passed", "no", ""])
        runner.review_report(self.path, lambda _: next(answers))
        report = json.loads(self.path.read_text())
        self.assertEqual(report["checks"], self.original["checks"])
        self.assertEqual(report["source_revision"], "abc")
        self.assertEqual(report["review"]["task_completion"], "pass")
        self.assertEqual(report["review"]["needed_external_tool"], "no")

    def test_invalid_review_leaves_report_unchanged(self):
        before = self.path.read_bytes()
        answers = iter(["pass", "pass", "pass", "invalid"])
        with self.assertRaises(ValueError):
            runner.review_report(self.path, lambda _: next(answers))
        self.assertEqual(self.path.read_bytes(), before)

    def test_blocked_report_cannot_be_reviewed_as_pass(self):
        data = {**self.original, "status": "blocked"}
        self.path.write_text(json.dumps(data))
        before = self.path.read_bytes()
        with self.assertRaises(ValueError):
            runner.review_report(self.path, lambda _: "pass")
        self.assertEqual(self.path.read_bytes(), before)
