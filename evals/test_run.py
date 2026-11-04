"""Check the grader against false passes; these tests never call a model."""
import importlib.util
import os
from pathlib import Path
import tempfile
import unittest
from unittest import mock

spec = importlib.util.spec_from_file_location("eval_runner", Path(__file__).with_name("run.py"))
runner = importlib.util.module_from_spec(spec)
spec.loader.exec_module(runner)


class EnvironmentIsolationTests(unittest.TestCase):
    def test_host_credentials_are_not_passed_to_the_app(self):
        with mock.patch.dict(os.environ, {
            "OPENROUTER_API_KEY": "sk-or-secret",
            "AWS_SECRET_ACCESS_KEY": "aws-secret",
            "GITHUB_TOKEN": "gh-secret",
            "CUSTOM_SERVICE_API_KEY": "custom-secret",
            "PATH": "/usr/bin",
        }, clear=True):
            env = runner.make_env(Path("/tmp/eval-case"), "model")
        self.assertEqual(env["PATH"], "/usr/bin")
        self.assertEqual(env["AH_API_KEY"], "lemonade-local")
        for key in ("OPENROUTER_API_KEY", "AWS_SECRET_ACCESS_KEY", "GITHUB_TOKEN", "CUSTOM_SERVICE_API_KEY"):
            self.assertNotIn(key, env)

    def test_verifier_environment_is_scrubbed(self):
        with mock.patch.dict(os.environ, {"OPENAI_API_KEY": "secret", "PATH": "/bin"}, clear=True):
            env = runner.sanitized_env()
        self.assertEqual(env, {"PATH": "/bin"})


class VerificationTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.workspace = Path(self.temp.name) / "workspace"
        self.case = runner.CASES["coding"]
        self.before = runner.prepare(self.workspace, self.case)

    def check(self):
        return runner.verify(self.workspace, self.case, self.before)["checks"]

    def test_correct_solution_with_unchanged_tests(self):
        (self.workspace / "names.py").write_text('def normalize_name(value):\n    return " ".join(word.capitalize() for word in value.split())\n')
        self.assertTrue(all(self.check().values()))

    def test_empty_write_does_not_pass(self):
        (self.workspace / "names.py").write_text("")
        self.assertFalse(self.check()["independent_tests_pass"])

    def test_replacing_tests_with_passing_stub_is_rejected(self):
        (self.workspace / "test_names.py").write_text("# no tests\n")
        checks = self.check()
        self.assertFalse(checks["independent_tests_pass"])
        self.assertFalse(checks["file_scope_respected"])

    def test_unrequested_file_is_a_scope_failure(self):
        (self.workspace / "extra.txt").write_text("not requested")
        self.assertFalse(self.check()["file_scope_respected"])
