import json
import subprocess
import unittest
from capture import bounded_capture
from protocol import parse_event, MAX_RECORD


class CaptureTests(unittest.TestCase):
    def test_flooded_stdout_or_stderr_is_bounded(self):
        for stream in ("stdout", "stderr"):
            with self.subTest(stream=stream), self.assertRaisesRegex(RuntimeError, "output exceeds"):
                bounded_capture(["/usr/bin/python", "-c",
                    f"import sys\nwhile True: sys.{stream}.write('x'*8192); sys.{stream}.flush()"], timeout=2)

    def test_idle_subprocess_hits_deadline(self):
        with self.assertRaises(subprocess.TimeoutExpired):
            bounded_capture(["/usr/bin/python", "-c", "import time; time.sleep(5)"], timeout=0.05)

    def test_verifier_input_and_exit_status_survive_capture(self):
        result = bounded_capture(["/usr/bin/python", "-c", "import sys; print(sys.stdin.read()); sys.exit(3)"], input="fixture")
        self.assertEqual(result.returncode, 3)
        self.assertEqual(result.stdout.strip(), "fixture")

    def test_malformed_observations_do_not_drive_keys(self):
        valid = {"schema_version": 1, "type": "text", "turn": 1, "text": "hello"}
        self.assertEqual(parse_event(json.dumps(valid)), valid)
        for value in ([], {}, {**valid, "turn": "1"}, {**valid, "text": []}, {**valid, "type": []}):
            with self.subTest(value=value), self.assertRaises(ValueError):
                parse_event(json.dumps(value))
        with self.assertRaises(ValueError):
            parse_event(b"x" * (MAX_RECORD + 1))

    def test_snapshot_rejects_special_files_and_large_sparse_files(self):
        import os
        import tempfile
        from pathlib import Path
        from run import snapshot
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            os.mkfifo(root / "pipe")
            with self.assertRaises(ValueError):
                snapshot(root)
            (root / "pipe").unlink()
            with (root / "large").open("wb") as file:
                file.truncate(5 * 1024 * 1024)
            with self.assertRaises(ValueError):
                snapshot(root)
