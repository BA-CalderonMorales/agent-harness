# Windows development

Use native Windows Go 1.26.9 or newer, GNU Make, and Git for Windows.
Install these in permanent locations and add their executable directories to
your user PATH. Reopen PowerShell after changing PATH.

From the repository directory:

```powershell
go version
make build
make run RUN_ARGS=--version
make test
make verify
make install
```

The Makefile uses Git for Windows Bash for shell recipes and Windows Go for
compilation. It builds `build/agent-harness.exe`. Set `GIT_BASH_ROOT` when Git
is installed outside its usual directory. No WSL installation is needed.

`make test` uses a Windows Job Object to limit memory and terminate leftover
child processes. `MEMGUARD_MB` defaults to 8192. The soft Go heap goal defaults
to 2GiB, configurable with `GOMEMLIMIT`. Tests that redirect the home
directory must set both `HOME` and `USERPROFILE`; use `testenv.SetHome`.

`make lint` requires `golangci-lint` on PATH. Mutation tests use a separately
pinned tool module and temporarily modify source: run them in a disposable
checkout. To check a specific function:

```powershell
$env:GO_MUTESTING_MATCH = 'unmarshalContentBlock'
make mutation-test MUTATION_TARGETS=./pkg/types
```

Run Linux checks in a disposable GitHub Codespace. Delete the Codespace after
validation to release its resources.

`make release` checks that the code version matches the latest published tag,
then builds. It is expected to fail after a version bump and before publication;
the PR and tag workflow in the release skill performs publication.
