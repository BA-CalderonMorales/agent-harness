"""Exercise branch pruning against disposable local Git repositories only."""
import os
from pathlib import Path
import subprocess
import tempfile

SCRIPT = Path(__file__).resolve().parents[1] / "prune-branches.sh"


def main():
    with tempfile.TemporaryDirectory(prefix="ah-prune-test-") as directory:
        root = Path(directory)
        env = {**os.environ, "GIT_CONFIG_GLOBAL": os.devnull,
               "GIT_CONFIG_NOSYSTEM": "1", "HOME": str(root)}
        # Do not let caller-supplied repository overrides escape the fixture.
        for key in list(env):
            if key.startswith("GIT_") and key not in {"GIT_CONFIG_GLOBAL", "GIT_CONFIG_NOSYSTEM"}:
                del env[key]
        work, remote = root / "work", root / "remote.git"

        def run(*args, cwd=None):
            return subprocess.run(args, cwd=cwd or work, env=env, check=True,
                                  stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True).stdout

        def exists(ref):
            return subprocess.run(["git", "show-ref", "--verify", "--quiet", ref],
                                  cwd=work, env=env).returncode == 0

        run("git", "init", "--bare", "--initial-branch=main", str(remote), cwd=root)
        run("git", "init", "--initial-branch=main", str(work), cwd=root)
        run("git", "config", "user.name", "Prune fixture")
        run("git", "config", "user.email", "fixture@example.invalid")
        run("git", "config", "core.hooksPath", os.devnull)
        run("git", "commit", "--allow-empty", "-m", "fixture base")
        run("git", "remote", "add", "origin", str(remote))
        for branch in ("develop", "release/0.3.0"):
            run("git", "branch", branch)
        run("git", "push", "--all", "origin")
        run("git", "push", "origin", "main:refs/heads/release/0.3.1", "main:refs/heads/feature/HEAD-note")
        run("git", "checkout", "-b", "feature/pending")
        run("git", "commit", "--allow-empty", "-m", "unmerged work")
        run("git", "push", "origin", "feature/pending")
        run("git", "checkout", "main")
        run("git", "fetch", "origin")
        run("git", "remote", "set-head", "origin", "main")
        run("git", "update-ref", "refs/remotes/upstream/kept", "main")

        before = run("git", "for-each-ref", "--format=%(refname) %(objectname)")
        preview = run("bash", str(SCRIPT), "--dry-run")
        assert "[would delete]" in preview and "release/0.3.0" in preview, preview
        assert "[would delete remote]" in preview and "origin/release/0.3.1" in preview, preview
        assert "origin/feature/HEAD-note" in preview, preview
        assert "origin/origin" not in preview and "upstream/kept" not in preview, preview
        assert before == run("git", "for-each-ref", "--format=%(refname) %(objectname)"), "dry-run changed refs"

        result = run("bash", str(SCRIPT))
        assert "[failed]" not in result, result
        for branch in ("release/0.3.0", "release/0.3.1", "feature/HEAD-note"):
            assert not exists("refs/heads/" + branch), branch
            assert not exists("refs/remotes/origin/" + branch), branch
            assert not run("git", "ls-remote", "--heads", "origin", "refs/heads/" + branch), branch
        for branch in ("main", "develop", "feature/pending"):
            assert exists("refs/heads/" + branch), branch
            assert exists("refs/remotes/origin/" + branch), branch
            assert run("git", "ls-remote", "--heads", "origin", "refs/heads/" + branch), branch
        assert exists("refs/remotes/upstream/kept")
        assert run("git", "symbolic-ref", "refs/remotes/origin/HEAD").strip() == "refs/remotes/origin/main"
        print("prune-branches: named previews, symbolic/other remote exclusion, protected/unmerged retention and actual pruning passed")


if __name__ == "__main__":
    main()
