"""Check evidence retention and interruption independently of model quality."""
import json
from pathlib import Path
import tempfile
import unittest
from unittest import mock

import unattended as driver


class EvidenceTests(unittest.TestCase):
    def test_partial_prose_is_retained_after_failure(self):
        report = {"case": "coding", "status": "fail", "scenario": {"prompt": "Fix the file."},
                  "automation_error": "deadline"}
        text = driver.review_text(report, [{"type": "text", "turn": 1, "text": "I found a problem."}])
        self.assertIn("I found a problem.", text)
        self.assertIn("deadline", text)
        self.assertIn("Fix the file.", text)

    def test_interrupt_during_preflight_preserves_summary_without_launching(self):
        with tempfile.TemporaryDirectory() as directory:
            with mock.patch.object(driver.base, "RESULTS", Path(directory)), \
                 mock.patch.object(driver.base, "source_state", return_value={"revision": "test", "dirty": False}), \
                 mock.patch.object(driver, "available", side_effect=KeyboardInterrupt), \
                 mock.patch.object(driver, "Terminal") as terminal:
                summary = driver.run_batch(["coding", "cancel"], "test")
            terminal.assert_not_called()
            self.assertEqual(summary["status"], "interrupted")
            self.assertEqual([c["status"] for c in summary["cases"]], ["not_run", "not_run"])
            self.assertEqual(json.loads((Path(summary["evidence"]) / "summary.json").read_text())["status"], "interrupted")

    def test_missing_isolation_is_blocked_without_build_or_inference(self):
        with tempfile.TemporaryDirectory() as directory:
            with mock.patch.object(driver.base, "RESULTS", Path(directory)), \
                 mock.patch.object(driver.base, "source_state", return_value={"revision": "test", "dirty": False}), \
                 mock.patch.object(driver, "available", side_effect=RuntimeError("missing boundary")), \
                 mock.patch.object(driver.base, "preflight") as preflight, \
                 mock.patch.object(driver.base, "build_app") as build:
                summary = driver.run_batch(["coding"], "test")
            preflight.assert_not_called()
            build.assert_not_called()
            self.assertEqual(summary["status"], "blocked")
            self.assertEqual(summary["cases"][0]["status"], "blocked")
