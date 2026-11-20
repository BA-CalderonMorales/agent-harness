"""Tests for scripts/verify/capture-lemonade-contracts.py against a fake local server.

No real Lemonade is needed: a stdlib HTTP server plays the part, so the
failure paths (missing model, failing chat, wrong overflow behavior) are
exercised deterministically.
"""
import json
import os
from http.server import BaseHTTPRequestHandler, HTTPServer
from pathlib import Path
import subprocess
import sys
import tempfile
import threading
import unittest

SCRIPT = Path(__file__).resolve().parent / "capture-lemonade-contracts.py"

MODELS = {"data": [
    {"id": "big-model", "context_length": 8192, "max_context_window": 8192},
    {"id": "small-model", "context_length": 512, "max_context_window": 8192},
]}
STREAM = b'data: {"id":"c1","choices":[{"delta":{"content":"ok"},"finish_reason":"stop"}]}\n\ndata: [DONE]\n\n'
OVERFLOW = b'data: {"error":{"code":400,"message":"Max length reached!"}}\n\n'


class Fake(BaseHTTPRequestHandler):
    models_status = 200
    chat_status = 200
    overflow_error = True

    def log_message(self, *args):
        pass

    def reply(self, status, ctype, body):
        self.send_response(status)
        self.send_header("Content-Type", ctype)
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def do_GET(self):
        self.reply(self.models_status, "application/json", json.dumps(MODELS).encode())

    def do_POST(self):
        body = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
        if self.chat_status != 200:
            self.reply(self.chat_status, "application/json", b'{"error":{"message":"model not found"}}')
        elif len(body["messages"][0]["content"]) > 1000:
            self.reply(200, "text/event-stream", OVERFLOW if self.overflow_error else STREAM)
        else:
            self.reply(200, "text/event-stream", STREAM)


def run(handler_attrs, *args):
    handler = type("H", (Fake,), handler_attrs)
    server = HTTPServer(("127.0.0.1", 0), handler)
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    try:
        env = dict(os.environ, LEMONADE_BASE_URL="http://127.0.0.1:%d/v1" % server.server_port,
                   LEMONADE_TIMEOUT="10")
        env.pop("LEMONADE_MODEL", None)
        with tempfile.TemporaryDirectory(prefix="ah-capture-test-") as out:
            target = Path(out) / "fixtures"
            proc = subprocess.run([sys.executable, "-I", str(SCRIPT), "--out", str(target), *args],
                                  capture_output=True, text=True, env=env)
            written = sorted(p.name for p in target.iterdir()) if target.exists() else []
            return proc, written
    finally:
        server.shutdown()
        server.server_close()


class CaptureTest(unittest.TestCase):
    def test_happy_path_writes_all_fixtures(self):
        proc, written = run({})
        self.assertEqual(proc.returncode, 0, proc.stderr)
        self.assertEqual(written, ["models.http", "overflow.http", "stream-text.http", "stream-tool-call.http"])
        self.assertIn("small-model", proc.stdout)  # picked the reduced-window model

    def test_explicit_model_is_used(self):
        proc, written = run({}, "--model", "big-model")
        self.assertEqual(proc.returncode, 0, proc.stderr)
        self.assertIn("big-model", proc.stdout)
        self.assertEqual(len(written), 4)

    def test_model_missing_from_models_writes_nothing(self):
        proc, written = run({}, "--model", "ghost-model")
        self.assertNotEqual(proc.returncode, 0)
        self.assertIn("ghost-model", proc.stderr)
        self.assertEqual(written, [])

    def test_no_identifiable_model_requires_explicit_model(self):
        saved = [dict(m, context_length=m["max_context_window"]) for m in MODELS["data"]]
        original = MODELS["data"]
        MODELS["data"] = saved
        try:
            proc, written = run({})
        finally:
            MODELS["data"] = original
        self.assertNotEqual(proc.returncode, 0)
        self.assertIn("--model", proc.stderr)
        self.assertEqual(written, [])

    def test_failing_chat_writes_nothing(self):
        proc, written = run({"chat_status": 404})
        self.assertNotEqual(proc.returncode, 0)
        self.assertIn("nothing written", proc.stderr)
        self.assertEqual(written, [])

    def test_missing_overflow_error_writes_nothing(self):
        proc, written = run({"overflow_error": False})
        self.assertNotEqual(proc.returncode, 0)
        self.assertEqual(written, [])

    def test_failing_models_writes_nothing(self):
        proc, written = run({"models_status": 500})
        self.assertNotEqual(proc.returncode, 0)
        self.assertEqual(written, [])


if __name__ == "__main__":
    unittest.main()
