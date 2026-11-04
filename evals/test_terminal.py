"""Exercise approval boundaries and OS isolation, without inference."""
from pathlib import Path
import os
import subprocess
import tempfile
import unittest

from terminal import allowed, available, sandbox, test_command as permits_test_command


class ApprovalTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.workspace = Path(self.temp.name) / "workspace"
        self.workspace.mkdir()

    def test_only_fixture_mutations_are_approved(self):
        for tool in ("write", "edit"):
            self.assertTrue(allowed(tool, {"file_path": "names.py"}, self.workspace, True))
            for path in ("test_names.py", "../names.py", "/etc/passwd", ""):
                self.assertFalse(allowed(tool, {"file_path": path}, self.workspace, True))
        (self.workspace / "names.py").symlink_to(self.workspace.parent / "outside.py")
        self.assertFalse(allowed("write", {"file_path": "names.py"}, self.workspace, True))
        self.assertFalse(allowed("write", None, self.workspace, True))
        self.assertFalse(allowed("agent", {}, self.workspace, True))
        self.assertFalse(allowed("read", {}, self.workspace, False))

    def test_unit_test_variations_without_shell_escape(self):
        for command in ("python -m unittest -v", "python3 -m unittest test_names -v",
                        "python -m unittest -v test_names.NormalizeNameTests.test_spaces",
                        f"cd '{self.workspace}' && python -m unittest -v test_names.NormalizeNameTests"):
            self.assertTrue(permits_test_command(command, self.workspace), command)
        for command in ("python -c 'print(1)'", "python -m unittest -v; touch bad",
                        "python -m unittest -v && touch bad", "python -m unittest $(touch bad)",
                        "python -m unittest `touch bad`", "python -m unittest -v > bad",
                        "python -m unittest -v\ntouch bad", "cd / && python -m unittest",
                        "python -m unittest attacker", "python -m unittest -v || true"):
            self.assertFalse(permits_test_command(command, self.workspace), command)

    def test_weather_fetches_require_known_https_sources(self):
        self.assertTrue(allowed("web_fetch", {"url": "https://api.weather.gov/points/41,-96"},
                                self.workspace, False, True))
        for url in ("http://api.weather.gov", "https://api.weather.gov.evil.invalid",
                    "https://user:secret@api.weather.gov", "https://127.0.0.1", "https://api.weather.gov:123"):
            self.assertFalse(allowed("web_fetch", {"url": url}, self.workspace, False, True), url)

    def test_verifier_can_write_fixture_but_cannot_read_host_home(self):
        try:
            available()
        except RuntimeError as error:
            self.skipTest(str(error))
        sentinel = self.workspace.parent / "private.txt"
        sentinel.write_text("host-only")
        # Expose a smaller, nested case so its sibling sentinel stays outside.
        case = self.workspace / "case"
        (case / "workspace").mkdir(parents=True)
        code = ("from pathlib import Path; "
                f"assert not Path({str(sentinel)!r}).exists(); "
                "Path('proof.txt').write_text('contained')")
        result = subprocess.run(sandbox(case, ["/usr/bin/python", "-c", code]), capture_output=True)
        self.assertEqual(result.returncode, 0, result.stderr.decode())
        self.assertEqual((case / "workspace/proof.txt").read_text(), "contained")

    def test_isolated_runtime_retains_system_certificate_roots(self):
        try:
            available()
        except RuntimeError as error:
            self.skipTest(str(error))
        case = self.workspace / "cert-case"
        (case / "workspace").mkdir(parents=True)
        result = subprocess.run(sandbox(case, ["/usr/bin/python", "-c",
                                "import ssl; assert ssl.create_default_context().get_ca_certs(), ssl.get_default_verify_paths()"]),
                                env={**os.environ, "SSL_CERT_FILE": "/unmounted/cert.pem"},
                                capture_output=True)
        self.assertEqual(result.returncode, 0, result.stderr.decode())

    def test_worker_cannot_read_or_overwrite_host_receipts(self):
        try:
            available()
        except RuntimeError as error:
            self.skipTest(str(error))
        case = self.workspace / "protected-case"
        (case / "workspace").mkdir(parents=True)
        receipt = case / "report.json"
        receipt.write_text("host-owned")
        code = ("from pathlib import Path; "
                f"p=Path({str(receipt)!r}); assert not p.exists(); print('receipt-hidden', flush=True); "
                "p.write_text('forged')")
        result = subprocess.run(sandbox(case, ["/usr/bin/python", "-c", code]), capture_output=True)
        self.assertIn(b"receipt-hidden", result.stdout, result.stderr.decode())
        self.assertEqual(receipt.read_text(), "host-owned")
