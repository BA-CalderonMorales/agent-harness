"""Exercise release installs offline, including upgrading an existing binary."""
import io
import os
from pathlib import Path
import subprocess
import tarfile
import tempfile
import unittest
import zipfile

INSTALLER = Path(__file__).resolve().parents[1] / 'install.sh'


class InstallerTest(unittest.TestCase):
    def run_install(self, platform, archive_suffix, existing=False):
        with tempfile.TemporaryDirectory() as root:
            root = Path(root)
            mocks = root / 'mocks'
            mocks.mkdir()
            target = root / 'new' / 'bin'
            binary = 'agent-harness.exe' if platform == 'MINGW64_NT' else 'agent-harness'
            if existing:
                target.mkdir(parents=True)
                (target / binary).write_text('old binary')
            settings = root / 'settings.json'
            settings.write_text('{"theme":"nord"}')
            payload = b'#!/bin/sh\necho v9.8.7\n'
            name = 'agent-harness-' + archive_suffix
            if platform == 'MINGW64_NT':
                archive = root / (name + '.zip')
                with zipfile.ZipFile(archive, 'w') as out:
                    out.writestr(name, payload)
            else:
                archive = root / (name + '.tar.gz')
                with tarfile.open(archive, 'w:gz') as out:
                    info = tarfile.TarInfo(name)
                    info.size = len(payload)
                    out.addfile(info, io.BytesIO(payload))
            commands = {
                'uname': '#!/bin/sh\nif [ "$1" = -s ]; then echo "$TEST_PLATFORM"; else echo x86_64; fi\n',
                # BSD grep does not support -P. Make the portability failure visible on Linux too.
                'grep': '#!/bin/sh\nexit 2\n',
                'curl': '''#!/bin/sh
case "$*" in
  *api.github.com*) printf '{"tag_name":"v9.8.7"}';;
  *) [ "$4" = "https://github.com/BA-CalderonMorales/agent-harness/releases/download/v9.8.7/$TEST_ARCHIVE_NAME" ] || exit 22
     cp "$TEST_ARCHIVE" "$3";;
esac
''',
                'unzip': '#!/usr/bin/env python3\nimport sys, zipfile\nwith zipfile.ZipFile(sys.argv[-1]) as archive: archive.extractall()\n',
                'sudo': '#!/bin/sh\necho unexpected sudo >&2\nexit 1\n',
            }
            for name, body in commands.items():
                script = mocks / name
                script.write_text(body)
                script.chmod(0o755)
            env = dict(os.environ, PATH=str(mocks) + ':' + os.environ['PATH'],
                       TEST_PLATFORM=platform, TEST_ARCHIVE=str(archive),
                       TEST_ARCHIVE_NAME=archive.name)
            result = subprocess.run(['bash', str(INSTALLER), '--dir', str(target)],
                                    env=env, capture_output=True, text=True)
            self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
            self.assertEqual((target / binary).read_bytes(), payload)
            self.assertTrue(os.access(target / binary, os.X_OK))
            self.assertIn('v9.8.7', result.stdout)
            self.assertEqual(settings.read_text(), '{"theme":"nord"}')

    def test_linux_new_directory(self):
        self.run_install('Linux', 'linux-amd64')

    def test_macos_upgrade(self):
        self.run_install('Darwin', 'darwin-amd64', existing=True)

    def test_windows_release_asset(self):
        self.run_install('MINGW64_NT', 'windows-amd64.exe')


if __name__ == '__main__':
    unittest.main()
