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

        def run(*args, check=True, extra_env=None):
            result = subprocess.run(args, cwd=work, env={**env, **(extra_env or {})},
                                    stdin=subprocess.DEVNULL, stdout=subprocess.PIPE,
                                    stderr=subprocess.STDOUT, text=True)
            if check and result.returncode != 0:
                raise AssertionError(f"{args} failed ({result.returncode}):\n{result.stdout}")
            return result

        def release(*args, check=True):
            return run("sh", str(SCRIPT), "--skip-ci-check", *args, check=check)

        # A fixture gh that answers the CI query with a chosen state and
        # records the arguments it was called with.
        fake_bin, gh_log = root / "bin", root / "gh-args"
        fake_bin.mkdir()
        fake_gh = fake_bin / "gh"
        fake_gh.write_text('#!/bin/sh\nprintf \'%s\\n\' "$*" >> "$FAKE_GH_LOG"\n'
                           'printf \'%s\\n\' "$FAKE_GH_STATE"\n')
        fake_gh.chmod(0o755)

        def release_with_ci(state, *args):
            return run("sh", str(SCRIPT), *args, check=False, extra_env={
                "PATH": f"{fake_bin}{os.pathsep}{env['PATH']}",
                "FAKE_GH_STATE": state, "FAKE_GH_LOG": str(gh_log)})

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

        # The default CI gate: only a completed, successful run on the
        # origin/main commit lets a release through.
        local_head = run("git", "rev-parse", "HEAD").stdout.strip()
        for state, reason in (("completed failure", "did not pass"),
                              ("in_progress ", "still running"),
                              ("null null", "no CI run found"),
                              ("", "no CI run found")):
            gated = release_with_ci(state, "--dry-run")
            assert gated.returncode != 0 and reason in gated.stdout, (state, gated.stdout)
        passed = release_with_ci("completed success", "--dry-run")
        assert passed.returncode == 0 and "Release v0.4.2" in passed.stdout, passed.stdout
        queried = gh_log.read_text()
        assert f"--commit {released_sha}" in queried and local_head not in queried, queried

        refused = release(check=False)
        assert refused.returncode != 0 and "pass --yes" in refused.stdout, refused.stdout
        assert not remote_tag("v0.4.2"), "a refused release pushed a tag"

        bad = release("--version", "0.5", check=False)
        assert bad.returncode != 0 and "--version must look like" in bad.stdout, bad.stdout

        existing = release("--version", "v0.4.1", check=False)
        assert existing.returncode != 0 and "already exists" in existing.stdout, existing.stdout

        # Inside GitHub Actions the tagged version goes to $GITHUB_OUTPUT for
        # the publish step, and only when a tag was actually pushed.
        actions_output = root / "github-output"
        actions_env = {"GITHUB_OUTPUT": str(actions_output)}
        bumped = run("sh", str(SCRIPT), "--skip-ci-check", "--dry-run", "--bump", "minor",
                     extra_env=actions_env).stdout
        assert "Release v0.5.0" in bumped, bumped
        assert not actions_output.exists(), "a dry run wrote a version for the publish step"

        done = run("sh", str(SCRIPT), "--skip-ci-check", "--yes", extra_env=actions_env).stdout
        assert "Pushed v0.4.2" in done, done
        assert remote_tag("v0.4.2") == released_sha, "tag does not point at origin/main"
        assert actions_output.read_text() == "version=v0.4.2\n", actions_output.read_text()

        again = release("--yes").stdout
        assert "Nothing to release" in again, again

        # An explicit --version is validated even when there is nothing new.
        invalid = release("--version", "v0.5.0-rc..1", check=False)
        assert invalid.returncode != 0 and "--version must look like" in invalid.stdout, invalid.stdout
        for empty in (("--version", ""), ("--version",)):
            missing = release(*empty, check=False)
            assert missing.returncode != 0 and "--version needs a value" in missing.stdout, missing.stdout

        # With no new commits, an explicit --version promotes the same commit.
        promoted = release("--yes", "--version", "v0.5.0-rc.1").stdout
        assert "Pushed v0.5.0-rc.1" in promoted, promoted
        assert remote_tag("v0.5.0-rc.1") == released_sha, "promotion did not tag origin/main"

        # A prerelease tag is never the base for the next automatic version.
        run("git", "checkout", "--quiet", "main")
        commit("fix(tui): after the candidate")
        run("git", "push", "--quiet", "origin", "main")
        after_rc = release("--dry-run").stdout
        assert "Release v0.4.3" in after_rc and "v0.4.2 + 1 commit(s)" in after_rc, after_rc

        # A non-conventional subject blocks an automatic bump, not an explicit one.
        commit("Add export")
        run("git", "push", "--quiet", "origin", "main")
        blocked = release("--dry-run", check=False)
        assert blocked.returncode != 0 and "not all conventional" in blocked.stdout, blocked.stdout
        chosen = release("--dry-run", "--bump", "minor").stdout
        assert "Release v0.5.0" in chosen, chosen

        print("release: dry run, origin/main targeting, prompt refusal, version validation,"
              " explicit bumps, tag push, nothing-to-release, same-commit promotion,"
              " stable-tag base and the conventional-commit gate passed")


if __name__ == "__main__":
    main()
