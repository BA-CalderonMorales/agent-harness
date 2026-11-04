"""Real binary + real terminal + scripted provider. No live inference or spend.

Run after make build with AH_EVAL_TEST_BINARY=/absolute/path/to/agent-harness.
These checks exercise UI commands, permissions, tools and independent grading.
"""
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import json
import os
from pathlib import Path
import tempfile
import threading
import time
import unittest
from unittest import mock

import unattended


class Provider(BaseHTTPRequestHandler):
    def log_message(self, *args):
        pass

    def do_GET(self):
        self.send_response(200)
        self.end_headers()
        self.wfile.write(b'{"data":[{"id":"journey-fixture","context_length":8192}]}')

    def do_POST(self):
        json.loads(self.rfile.read(int(self.headers["Content-Length"])))
        self.server.calls += 1
        call = self.server.calls
        self.send_response(200)
        self.send_header("Content-Type", "text/event-stream")
        self.end_headers()

        def send(delta, reason=None):
            payload = {"choices": [{"delta": delta, "finish_reason": reason}]}
            self.wfile.write(("data: " + json.dumps(payload) + "\n\n").encode())
            self.wfile.flush()

        def tool(name, inputs):
            send({"tool_calls": [{"index": 0, "id": "call-" + str(call), "type": "function",
                                  "function": {"name": name, "arguments": json.dumps(inputs)}}]})
            send({}, "tool_calls")

        try:
            if getattr(self.server, "generated_case", None):
                case = self.server.generated_case
                if call == 1:
                    tool("read", {"file_path": case["allowed_files"][0]})
                elif call == 2:
                    tool("read", {"file_path": case["test_module"] + ".py"})
                elif call == 3:
                    tool("write", {"file_path": case["allowed_files"][0], "content": self.server.solution})
                elif call == 4:
                    tool("bash", {"command": "python -m unittest -v"})
                else:
                    send({"content": "Public examples passed. Implementation complete."})
                    send({}, "stop")
            elif self.server.scenario == "stall":
                time.sleep(3)
                send({"content": "Too late"})
                send({}, "stop")
            elif self.server.scenario == "failed_tests":
                if call == 1:
                    tool("bash", {"command": "python -m unittest -v"})
                else:
                    send({"content": "The tests failed; implementation is unfinished."})
                    send({}, "stop")
            elif self.server.scenario == "denied_write":
                if call == 1:
                    tool("write", {"file_path": "test_names.py", "content": "# bypass"})
                else:
                    send({"content": "The proposed test change was denied."})
                    send({}, "stop")
            elif self.server.scenario == "coding":
                if call == 1:
                    tool("read", {"file_path": "names.py"})
                elif call == 2:
                    tool("read", {"file_path": "test_names.py"})
                elif call == 3:
                    tool("write", {"file_path": "names.py", "content":
                         'def normalize_name(value):\n    return " ".join(w.capitalize() for w in value.split())\n'})
                elif call == 4:
                    tool("bash", {"command": "python -m unittest -v"})
                else:
                    send({"content": "All four tests passed; only names.py changed."})
                    send({}, "stop")
            elif self.server.scenario == "cancel" and call == 1:
                send({"content": "Starting the explanation..."})
                time.sleep(2)
                send({"content": "Late text must not overwrite the next turn."})
                send({}, "stop")
            else:
                text = "READY_TO_CONTINUE" if self.server.scenario == "cancel" else "Hello from the actual app."
                send({"content": text})
                send({}, "stop")
            self.wfile.write(b"data: [DONE]\n\n")
        except (BrokenPipeError, ConnectionResetError):
            pass


@unittest.skipUnless(os.environ.get("AH_EVAL_TEST_BINARY"), "set AH_EVAL_TEST_BINARY for real terminal integration")
class JourneyTests(unittest.TestCase):
    def exercise(self, name, scenario=None, expected_status="needs_review", timeout=25, case=None, solution=None):
        binary = Path(os.environ["AH_EVAL_TEST_BINARY"]).resolve()
        with tempfile.TemporaryDirectory() as directory:
            server = ThreadingHTTPServer(("127.0.0.1", 0), Provider)
            server.calls, server.scenario = 0, scenario or name
            server.generated_case, server.solution = case, solution
            threading.Thread(target=server.serve_forever, daemon=True).start()
            try:
                with mock.patch.object(unattended.base, "ENDPOINT", f"http://127.0.0.1:{server.server_port}/v1"):
                    report = unattended.run_case(name, Path(directory) / name, binary,
                                                 "journey-fixture", {"revision": "test", "dirty": False}, timeout,
                                                 case_override=case)
                self.assertEqual(report["status"], expected_status, report)
                if expected_status == "needs_review":
                    self.assertTrue(all(report["verification"]["checks"].values()))
                self.assertTrue((Path(directory) / name / "terminal.ansi").stat().st_size)
                review = (Path(directory) / name / "review.md").read_text()
                self.assertIn("Actual Agent Harness output", review)
                if scenario == "failed_tests":
                    self.assertIn("Tool result: error", review)
                    self.assertIn("exit status", review)
                if scenario == "denied_write":
                    self.assertEqual((Path(directory) / name / "workspace/test_names.py").read_text(), unattended.base.TESTS)
                return report
            finally:
                server.shutdown()
                server.server_close()

    def test_greeting(self):
        self.assertEqual(self.exercise("greeting")["final_response"], "Hello from the actual app.")

    def test_read_write_execute_and_verify(self):
        report = self.exercise("coding")
        self.assertEqual(report["verification"]["changed_files"], ["names.py"])
        self.assertTrue(report["approval_decisions"])
        self.assertTrue(all(d["approved"] for d in report["approval_decisions"]))

    def test_cancel_and_continue(self):
        self.assertEqual(self.exercise("cancel")["final_response"], "READY_TO_CONTINUE")

    def test_failed_tests_are_visible_as_error_in_the_actual_tui(self):
        report = self.exercise("coding", "failed_tests", "fail")
        self.assertFalse(report["verification"]["checks"]["independent_tests_pass"])

    def test_forbidden_mutation_is_denied_without_changing_tests(self):
        report = self.exercise("coding", "denied_write", "fail")
        self.assertEqual([d["approved"] for d in report["approval_decisions"]], [False])

    def test_deadline_stops_child_and_retains_evidence(self):
        report = self.exercise("greeting", "stall", "fail", timeout=1)
        self.assertIn("deadline", report["automation_error"])
        self.assertIsNotNone(report["driver_shutdown_exit_code"])

    def test_varied_families_use_real_tui_and_private_host_checks(self):
        from suite import scenario, FAMILIES
        from test_suite import SOLUTIONS
        for family in FAMILIES:
            with self.subTest(family=family):
                case = scenario(family, 7)
                report = self.exercise(family, case=case, solution=SOLUTIONS[family])
                self.assertTrue(report["verification"]["checks"]["hidden_checks_pass"])
                self.assertNotIn("private_inputs", report["scenario"])
                self.assertEqual(len(report["resources"]["limits"]["cpus"]), 1)
