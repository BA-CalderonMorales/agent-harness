"""Exercise scripts/release/release.sh against disposable local Git repositories only."""
import os
from pathlib import Path
import subprocess
import tempfile

SCRIPT = Path(__file__).resolve().parents[1] / "release" / "release.sh"


def main():
    with tempfile.TemporaryDirectory(prefix="ah-release-test-") as directory:
        root = Path(directory)
        env = {**os.environ, "GIT_CONFIG_GLOBAL": os.devnull,
               "GIT_CONFIG_NOSYSTEM": "1", "HOME": str(root)}
        # Do not let caller-supplied repository overrides escape the fixture.
        for key in list(env):
            if key.startswith("GIT_") and key not in {"GIT_CONFIG_GLOBAL", "GIT_CONFIG_NOSYSTEM"}:
                del env[key]
        work, remote = root / "work", root / "remote.git"

        def run(*args, check=True):
            result = subprocess.run(args, cwd=work, env=env, stdin=subprocess.DEVNULL,
                                    stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True)
            if check and result.returncode != 0:
                raise AssertionError(f"{args} failed ({result.returncode}):\n{result.stdout}")
            return result

        def release(*args, check=True):
            return run("sh", str(SCRIPT), "--skip-ci-check", *args, check=check)

        def remote_tag(tag):
            out = run("git", "ls-remote", "--tags", "origin", f"refs/tags/{tag}^{{}}").stdout
            return out.split()[0] if out else ""

        def commit(message):
            run("git", "commit", "--allow-empty", "-m", message)

        subprocess.run(["git", "init", "--bare", "--initial-branch=main", str(remote)],
                       env=env, check=True, capture_output=True)
        subprocess.run(["git", "init", "--initial-branch=main", str(work)],
                       env=env, check=True, capture_output=True)
        run("git", "config", "user.name", "Release fixture")
        run("git", "config", "user.email", "fixture@example.invalid")
        run("git", "config", "core.hooksPath", os.devnull)
        run("git", "remote", "add", "origin", str(remote))
        commit("chore: initial")
        run("git", "tag", "-a", "v0.4.1", "-m", "v0.4.1")
        commit("feat(tui): a feature")
        commit("fix(config): a fix")
        run("git", "push", "--quiet", "origin", "main", "v0.4.1")
        released_sha = run("git", "rev-parse", "HEAD").stdout.strip()

        # Local work that is not on origin/main must never be tagged.
        run("git", "checkout", "--quiet", "-b", "feat/unpushed")
        commit("feat!: local only")

        before = run("git", "for-each-ref", "--format=%(refname) %(objectname)").stdout
        preview = release("--dry-run").stdout
        assert "Release v0.4.2 from origin/main" in preview, preview
        assert "v0.4.1 + 2 commit(s)" in preview, preview
        assert "local only" not in preview, preview
        assert "Dry run" in preview, preview
        after = run("git", "for-each-ref", "--format=%(refname) %(objectname)").stdout
        assert before == after, "dry run changed refs"

        refused = release(check=False)
        assert refused.returncode != 0 and "pass --yes" in refused.stdout, refused.stdout
        assert not remote_tag("v0.4.2"), "a refused release pushed a tag"

        bad = release("--version", "0.5", check=False)
        assert bad.returncode != 0 and "--version must look like" in bad.stdout, bad.stdout

        existing = release("--version", "v0.4.1", check=False)
        assert existing.returncode != 0 and "already exists" in existing.stdout, existing.stdout

        bumped = release("--dry-run", "--bump", "minor").stdout
        assert "Release v0.5.0" in bumped, bumped

        done = release("--yes").stdout
        assert "Pushed v0.4.2" in done, done
        assert remote_tag("v0.4.2") == released_sha, "tag does not point at origin/main"

        again = release("--yes").stdout
        assert "Nothing to release" in again, again

        print("release: dry run, origin/main targeting, prompt refusal, version validation,"
              " explicit bumps, tag push and nothing-to-release passed")


if __name__ == "__main__":
    main()
