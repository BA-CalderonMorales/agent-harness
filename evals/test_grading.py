from pathlib import Path
import tempfile
import unittest
import run as base
from grading import isolated_verify
from terminal import available


class GraderTests(unittest.TestCase):
    def test_candidate_python_cannot_change_worker_report(self):
        try:
            available()
        except RuntimeError as error:
            self.skipTest(str(error))
        with tempfile.TemporaryDirectory() as directory:
            case = Path(directory)
            work = case / "workspace"
            before = base.prepare(work, base.CASES["coding"])
            receipt = case / "report.json"
            receipt.write_text("original")
            (work / "names.py").write_text(
                f"from pathlib import Path\nPath({str(receipt)!r}).write_text('forged')\n"
                "def normalize_name(value): return value\n")
            result = isolated_verify(base, work, base.CASES["coding"], before)
            self.assertFalse(result["checks"]["independent_tests_pass"])
            self.assertEqual(receipt.read_text(), "original")

    def test_host_rejects_symlink_before_reading_candidate_source(self):
        with tempfile.TemporaryDirectory() as directory:
            work = Path(directory) / "workspace"
            before = base.prepare(work, base.CASES["coding"])
            (work / "names.py").unlink()
            (work / "names.py").symlink_to("/etc/passwd")
            self.assertFalse(isolated_verify(base, work, base.CASES["coding"], before)["checks"]["no_symlinks"])

    def test_verifier_cannot_connect_to_host_listener(self):
        import socket
        import subprocess
        from terminal import sandbox
        try:
            available()
        except RuntimeError as error:
            self.skipTest(str(error))
        with socket.socket() as listener, tempfile.TemporaryDirectory() as directory:
            listener.bind(("127.0.0.1", 0))
            listener.listen()
            (Path(directory) / "workspace").mkdir()
            code = ("import socket\ns=socket.socket(); s.settimeout(.2)\n"
                    f"assert s.connect_ex(('127.0.0.1', {listener.getsockname()[1]})) != 0\n")
            result = subprocess.run(sandbox(directory, ["/usr/bin/python3", "-c", code], network=False), capture_output=True)
            self.assertEqual(result.returncode, 0, result.stderr.decode())
